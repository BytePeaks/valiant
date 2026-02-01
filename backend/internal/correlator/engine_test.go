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

func TestAnalyzeImpact_IntentExecutionLinking(t *testing.T) {
	cfg := &config.Config{}
	cfg.Analysis.IntentExecutionCorrelationDur = 1 * time.Hour
	metrics := &ControllableMetrics{Calls: []domain.MetricValues{{}, {}}}
	eventTime := time.Now().Add(-2 * time.Hour)

	t.Run("Linked GitOps event", func(t *testing.T) {
		store := &MockStorage{
			changeEvents: []domain.ChangeEvent{
				{ID: "ci-evt-1", TriggerType: "CI", Metadata: map[string]string{"git_commit_sha": "abcdef123"}},
			},
		}
		engine := correlator.NewEngine(store, metrics, cfg)
		event := domain.ChangeEvent{
			ID:          "exec-evt-1",
			TriggerType: "GitOps",
			Timestamp:   eventTime,
			Metadata:    map[string]string{"git_commit_sha": "abcdef123"},
		}
		analysis, err := engine.AnalyzeImpact(context.Background(), event)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if analysis.IsOrphaned {
			t.Error("Expected IsOrphaned to be false, but it was true")
		}
	})

	t.Run("Orphaned GitOps event (no match)", func(t *testing.T) {
		store := &MockStorage{changeEvents: []domain.ChangeEvent{}}
		engine := correlator.NewEngine(store, metrics, cfg)
		event := domain.ChangeEvent{
			ID:          "exec-evt-2",
			TriggerType: "GitOps",
			Timestamp:   eventTime,
			Metadata:    map[string]string{"git_commit_sha": "abcdef123"},
		}
		analysis, err := engine.AnalyzeImpact(context.Background(), event)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !analysis.IsOrphaned {
			t.Error("Expected IsOrphaned to be true, but it was false")
		}
	})

	t.Run("Orphaned GitOps event (no metadata)", func(t *testing.T) {
		store := &MockStorage{}
		engine := correlator.NewEngine(store, metrics, cfg)
		event := domain.ChangeEvent{
			ID:          "exec-evt-3",
			TriggerType: "GitOps",
			Timestamp:   eventTime,
			Metadata:    map[string]string{},
		}
		analysis, err := engine.AnalyzeImpact(context.Background(), event)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !analysis.IsOrphaned {
			t.Error("Expected IsOrphaned to be true for event with no linking metadata, but it was false")
		}
	})

	t.Run("Orphaned manual event", func(t *testing.T) {
		store := &MockStorage{}
		engine := correlator.NewEngine(store, metrics, cfg)
		event := domain.ChangeEvent{
			ID:          "manual-evt-1",
			TriggerType: "manual",
			Timestamp:   eventTime,
		}
		analysis, err := engine.AnalyzeImpact(context.Background(), event)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !analysis.IsOrphaned {
			t.Error("Expected IsOrphaned to be true for manual event, but it was false")
		}
	})

	t.Run("CI event is never orphaned", func(t *testing.T) {
		store := &MockStorage{}
		engine := correlator.NewEngine(store, metrics, cfg)
		event := domain.ChangeEvent{
			ID:          "ci-evt-only",
			TriggerType: "CI",
			Timestamp:   eventTime,
		}
		analysis, err := engine.AnalyzeImpact(context.Background(), event)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if analysis.IsOrphaned {
			t.Error("Expected IsOrphaned to be false for a CI event, but it was true")
		}
	})
}

