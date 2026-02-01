package correlator_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"valiant/internal/config"
	"valiant/internal/correlator"
	"valiant/internal/domain"
)

// MockStorage implements storage.Storage
type MockStorage struct {
	snapshot     *domain.ImpactAnalysis
	changeEvents []domain.ChangeEvent
}

func (m *MockStorage) SaveChangeEvent(ctx context.Context, event domain.ChangeEvent) error { return nil }
func (m *MockStorage) GetChangeEvents(ctx context.Context, filters map[string]interface{}) ([]domain.ChangeEvent, error) {
	if m.changeEvents != nil {
		return m.changeEvents, nil
	}
	return nil, nil
}
func (m *MockStorage) GetChangeEventByID(ctx context.Context, id string) (domain.ChangeEvent, error) {
	return domain.ChangeEvent{}, nil
}
func (m *MockStorage) GetServices(ctx context.Context) ([]string, error) { return nil, nil }
func (m *MockStorage) GetEventsPendingAnalysis(ctx context.Context) ([]domain.ChangeEvent, error) {
	return nil, nil
}

func (m *MockStorage) SaveImpactAnalysis(ctx context.Context, analysis domain.ImpactAnalysis) error {
	m.snapshot = &analysis
	return nil
}

func (m *MockStorage) GetImpactAnalysisByEventID(ctx context.Context, eventID string) (*domain.ImpactAnalysis, error) {
	if m.snapshot != nil && m.snapshot.ChangeEvent.ID == eventID {
		return m.snapshot, nil
	}
	return nil, nil // Not found
}

// MockMetrics implements metrics.MetricsProvider
type MockMetrics struct {
	baseline domain.MetricValues
	impact   domain.MetricValues
}

func (m *MockMetrics) GetAverageMetrics(ctx context.Context, services []string, start, end time.Time) (domain.MetricValues, error) {
	// Simple heuristic: if start time is "old" (baseline), return baseline
	// In reality, tests will control this via the call order or context, but for simple unit test:
	// We can check the duration between start/end to guess, or just alternate.
	// A better way for deterministic testing is to define behavior based on the *window*.
	// But since the Engine calls Baseline first, then Impact...
	
	// Let's just return fixed values for the test case setup.
	// This mock is a bit simplistic; for a real test we might want checking arguments.
	return m.baseline, nil
}

// Better MockMetrics that allows specifying return values for specific calls
type ControllableMetrics struct {
	Calls []domain.MetricValues
	Index int
}

func (m *ControllableMetrics) GetAverageMetrics(ctx context.Context, services []string, start, end time.Time) (domain.MetricValues, error) {
	if m.Index >= len(m.Calls) {
		return domain.MetricValues{}, nil
	}
	val := m.Calls[m.Index]
	m.Index++
	return val, nil
}

func TestAnalyzeImpact_WindowNotClosed(t *testing.T) {
	cfg := &config.Config{}
	cfg.Analysis.BaselineDur = 30 * time.Minute
	cfg.Analysis.ImpactDur = 30 * time.Minute

	store := &MockStorage{}
	metrics := &MockMetrics{}
	engine := correlator.NewEngine(store, metrics, cfg)

	// Event happened just now
	event := domain.ChangeEvent{
		ID:        "evt-1",
		Timestamp: time.Now(),
	}

	_, err := engine.AnalyzeImpact(context.Background(), event)
	if !errors.Is(err, correlator.ErrImpactWindowNotClosed) {
		t.Errorf("expected ErrImpactWindowNotClosed, got %v", err)
	}
}

func TestAnalyzeImpact_CalculatesAndSaves(t *testing.T) {
	cfg := &config.Config{}
	cfg.Analysis.BaselineDur = 30 * time.Minute
	cfg.Analysis.ImpactDur = 30 * time.Minute

	store := &MockStorage{}
	
	// Setup metrics: Baseline (good), Impact (bad errors AND latency)
	metrics := &ControllableMetrics{
		Calls: []domain.MetricValues{
			// Baseline (Non-zero errors to allow >100% delta)
			{ErrorRate: 0.01, LatencyP95: 100, RPS: 100}, 
			// Impact: 
			// Errors: 0.01 -> 0.05 (Delta 4.0). Norm = min(4.0/2.0, 1.0) = 1.0. Contrib = 0.4.
			// Latency: 100 -> 300 (Delta 2.0). Norm = min(2.0/2.0, 1.0) = 1.0. Contrib = 0.3.
			// Total Score = 0.4 + 0.3 = 0.7 (HIGH)
			{ErrorRate: 0.05, LatencyP95: 300, RPS: 100}, 
		},
	}

	engine := correlator.NewEngine(store, metrics, cfg)

	// Event happened 1 hour ago (window closed)
	event := domain.ChangeEvent{
		ID:        "evt-2",
		Timestamp: time.Now().Add(-1 * time.Hour),
	}

	analysis, err := engine.AnalyzeImpact(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify Impact Level
	if analysis.ImpactLevel != "HIGH" {
		t.Errorf("expected ImpactLevel HIGH, got %s (Score: %f)", analysis.ImpactLevel, analysis.ImpactScore)
	}

	// Verify it was saved to storage
	if store.snapshot == nil {
		t.Error("expected snapshot to be saved")
	}
}

func TestAnalyzeImpact_LowConfidence(t *testing.T) {
	cfg := &config.Config{}
	store := &MockStorage{}
	
	// Low RPS scenario
	metrics := &ControllableMetrics{
		Calls: []domain.MetricValues{
			{RPS: 0.5, ErrorRate: 0}, // Baseline
			{RPS: 0.8, ErrorRate: 0.5}, // Impact (High error rate but low volume)
		},
	}

	engine := correlator.NewEngine(store, metrics, cfg)
	event := domain.ChangeEvent{ID: "evt-low-rps", Timestamp: time.Now().Add(-1 * time.Hour)}

	analysis, _ := engine.AnalyzeImpact(context.Background(), event)

	if analysis.ConfidenceScore >= 1.0 {
		t.Errorf("expected low confidence score (< 1.0) for low RPS, got %f", analysis.ConfidenceScore)
	}
}

func TestAnalyzeImpact_PerfectStability(t *testing.T) {
	cfg := &config.Config{}
	store := &MockStorage{}
	metrics := &ControllableMetrics{
		Calls: []domain.MetricValues{
			{ErrorRate: 0.01, LatencyP95: 100, RPS: 50}, // Baseline
			{ErrorRate: 0.01, LatencyP95: 100, RPS: 50}, // Impact (Identical)
		},
	}

	engine := correlator.NewEngine(store, metrics, cfg)
	event := domain.ChangeEvent{ID: "evt-stable", Timestamp: time.Now().Add(-2 * time.Hour)}

	analysis, err := engine.AnalyzeImpact(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if analysis.ImpactScore != 0.0 {
		t.Errorf("expected 0.0 score for perfect stability, got %f", analysis.ImpactScore)
	}
	if analysis.ImpactLevel != "NONE" {
		t.Errorf("expected NONE level, got %s", analysis.ImpactLevel)
	}
}

func TestAnalyzeImpact_RPSDrop(t *testing.T) {
	cfg := &config.Config{}
	store := &MockStorage{}
	metrics := &ControllableMetrics{
		Calls: []domain.MetricValues{
			{RPS: 100}, // Baseline
			{RPS: 0},   // Impact (100% drop)
		},
	}

	engine := correlator.NewEngine(store, metrics, cfg)
	event := domain.ChangeEvent{ID: "evt-crash", Timestamp: time.Now().Add(-2 * time.Hour)}

	analysis, _ := engine.AnalyzeImpact(context.Background(), event)

	// RPS drop of 100% should contribute 1.0 * weightRPS (0.1) = 0.1
	// This might be LOW impact depending on threshold (0.1 is LOW threshold)
	if analysis.ImpactScore < 0.1 {
		t.Errorf("expected at least 0.1 score for 100%% RPS drop, got %f", analysis.ImpactScore)
	}
}

func TestAnalyzeImpact_ZeroBaseline(t *testing.T) {
	cfg := &config.Config{}
	store := &MockStorage{}
	metrics := &ControllableMetrics{
		Calls: []domain.MetricValues{
			{ErrorRate: 0},   // Baseline (0 errors)
			{ErrorRate: 0.5}, // Impact (50% error rate - massive spike)
		},
	}

	engine := correlator.NewEngine(store, metrics, cfg)
	event := domain.ChangeEvent{ID: "evt-zero-start", Timestamp: time.Now().Add(-2 * time.Hour)}

	analysis, _ := engine.AnalyzeImpact(context.Background(), event)

	// Logic: if baseline 0 and impact > 0, delta is 1.0 (100% increase cap)
	// Normalized error score = min(1.0/2.0, 1.0) = 0.5
	// Weighted score = 0.5 * 0.4 = 0.2
	expectedScore := 0.2
	if analysis.ImpactScore != expectedScore {
		t.Errorf("expected score %f for zero baseline spike, got %f", expectedScore, analysis.ImpactScore)
	}
}

func TestAnalyzeImpact_PrometheusError(t *testing.T) {
	cfg := &config.Config{}
	store := &MockStorage{}
	
	// Mock metrics returns error
	metrics := &ErrorMetrics{err: errors.New("prometheus connection refused")}
	
	engine := correlator.NewEngine(store, metrics, cfg)
	event := domain.ChangeEvent{ID: "evt-err", Timestamp: time.Now().Add(-1 * time.Hour)}

	_, err := engine.AnalyzeImpact(context.Background(), event)
	if err == nil {
		t.Error("expected error when metrics provider fails, got nil")
	}
}

type ErrorMetrics struct {
	err error
}
func (m *ErrorMetrics) GetAverageMetrics(ctx context.Context, s []string, start, end time.Time) (domain.MetricValues, error) {
	return domain.MetricValues{}, m.err
}

func TestAnalyzeImpact_NoServices(t *testing.T) {
	cfg := &config.Config{}
	store := &MockStorage{}
	metrics := &MockMetrics{baseline: domain.MetricValues{RPS: 10}}
	
	engine := correlator.NewEngine(store, metrics, cfg)
	// Event with empty affected_services
	event := domain.ChangeEvent{
		ID:               "evt-no-svc", 
		Timestamp:        time.Now().Add(-1 * time.Hour),
		AffectedServices: []string{},
	}

	analysis, _ := engine.AnalyzeImpact(context.Background(), event)

	// Since we query without service filter, we still get data (from mock),
	// but confidence should potentially be lower if we decided that was a signal.
	// Current logic only uses RPS for confidence.
	if analysis.ConfidenceScore < 1.0 {
		t.Errorf("expected 1.0 confidence for healthy RPS, even with no services, got %f", analysis.ConfidenceScore)
	}
}

func TestAnalyzeImpact_InstantRollout(t *testing.T) {
	cfg := &config.Config{}
	store := &MockStorage{}
	metrics := &ControllableMetrics{
		Calls: []domain.MetricValues{{RPS: 10}, {RPS: 10}},
	}
	
	engine := correlator.NewEngine(store, metrics, cfg)
	
	// Start and End times are identical
	now := time.Now().Add(-1 * time.Hour)
	event := domain.ChangeEvent{
		ID:        "evt-instant",
		Timestamp: now,
		EndTime:   &now,
	}

	_, err := engine.AnalyzeImpact(context.Background(), event)
	if err != nil {
		t.Errorf("unexpected error for instant rollout: %v", err)
	}
}

func TestAnalyzeImpact_OrphanEvent(t *testing.T) {
	cfg := &config.Config{}
	cfg.Analysis.OrphanCorrelationDur = 1 * time.Hour
	// Metrics will be called for baseline/impact, so provide dummy values
	metrics := &ControllableMetrics{
		Calls: []domain.MetricValues{{}, {}},
	}
	eventTime := time.Now().Add(-2 * time.Hour) // Far enough in past

	// --- SCENARIO 1: Is Orphaned ---
	t.Run("IsOrphaned", func(t *testing.T) {
		store := &MockStorage{
			changeEvents: []domain.ChangeEvent{}, // No corresponding CI event
		}
		engine := correlator.NewEngine(store, metrics, cfg)
		event := domain.ChangeEvent{
			ID:               "evt-orphan",
			Timestamp:        eventTime,
			TriggerType:      "GitOps",
			AffectedServices: []string{"service-a"},
		}

		analysis, err := engine.AnalyzeImpact(context.Background(), event)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !analysis.IsOrphaned {
			t.Error("expected IsOrphaned to be true, but it was false")
		}
	})

	// --- SCENARIO 2: Not Orphaned ---
	t.Run("NotOrphaned", func(t *testing.T) {
		store := &MockStorage{
			changeEvents: []domain.ChangeEvent{ // Found a matching CI event
				{ID: "evt-ci-match", TriggerType: "CI"},
			},
		}
		engine := correlator.NewEngine(store, metrics, cfg)
		event := domain.ChangeEvent{
			ID:               "evt-not-orphan",
			Timestamp:        eventTime,
			TriggerType:      "GitOps",
			AffectedServices: []string{"service-a"},
		}

		analysis, err := engine.AnalyzeImpact(context.Background(), event)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if analysis.IsOrphaned {
			t.Error("expected IsOrphaned to be false, but it was true")
		}
	})

	// --- SCENARIO 3: Non-execution events are never orphaned ---
	t.Run("NeverOrphanedForCI", func(t *testing.T) {
		store := &MockStorage{
			changeEvents: []domain.ChangeEvent{}, // No other events exist
		}
		engine := correlator.NewEngine(store, metrics, cfg)
		event := domain.ChangeEvent{
			ID:               "evt-ci-never-orphan",
			Timestamp:        eventTime,
			TriggerType:      "CI", // This is not an execution event
			AffectedServices: []string{"service-a"},
		}

		analysis, err := engine.AnalyzeImpact(context.Background(), event)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if analysis.IsOrphaned {
			t.Error("expected IsOrphaned to be false for a CI event, but it was true")
		}
	})

	// --- SCENARIO 4: Match at the edge of the window ---
	t.Run("MatchAtWindowEdge", func(t *testing.T) {
		store := &MockStorage{
			// The GetChangeEvents mock in the real implementation would handle the time filtering,
			// for this test, we simulate that it *does* return an event that is precisely on the boundary.
			changeEvents: []domain.ChangeEvent{{ID: "evt-ci-edge", TriggerType: "CI"}},
		}
		engine := correlator.NewEngine(store, metrics, cfg)
		event := domain.ChangeEvent{
			ID:               "evt-edge-case",
			Timestamp:        eventTime,
			TriggerType:      "GitOps",
			AffectedServices: []string{"service-a"},
		}

		analysis, err := engine.AnalyzeImpact(context.Background(), event)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if analysis.IsOrphaned {
			t.Error("expected IsOrphaned to be false when match is at window edge, but it was true")
		}
	})

	// --- SCENARIO 5: No match just outside the window ---
	t.Run("NoMatchOutsideWindow", func(t *testing.T) {
		store := &MockStorage{
			// Simulate that the storage query found no events in the window.
			changeEvents: []domain.ChangeEvent{},
		}
		engine := correlator.NewEngine(store, metrics, cfg)
		event := domain.ChangeEvent{
			ID:               "evt-outside-window",
			Timestamp:        eventTime,
			TriggerType:      "GitOps",
			AffectedServices: []string{"service-a"},
		}

		analysis, err := engine.AnalyzeImpact(context.Background(), event)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !analysis.IsOrphaned {
			t.Error("expected IsOrphaned to be true when match is outside window, but it was false")
		}
	})

	// --- SCENARIO 6: Orphaned when services do not overlap ---
	t.Run("OrphanedWithMismatchedServices", func(t *testing.T) {
		store := &MockStorage{
			// Storage returns a CI event, but the correlator's filter *should* have excluded it.
			// We simulate this by having the mock return an empty list.
			changeEvents: []domain.ChangeEvent{},
		}
		engine := correlator.NewEngine(store, metrics, cfg)
		event := domain.ChangeEvent{
			ID:               "evt-mismatched-svc",
			Timestamp:        eventTime,
			TriggerType:      "GitOps",
			AffectedServices: []string{"service-a"},
		}

		analysis, err := engine.AnalyzeImpact(context.Background(), event)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !analysis.IsOrphaned {
			t.Error("expected IsOrphaned to be true when services do not match, but it was false")
		}
	})
}
