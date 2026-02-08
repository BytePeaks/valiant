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
func (m *MockStorage) GetChangeEvents(ctx context.Context, filters map[string]interface{}) ([]domain.ChangeEvent, int, error) {
	if m.changeEvents != nil {
		return m.changeEvents, len(m.changeEvents), nil
	}
	return nil, 0, nil
}
func (m *MockStorage) DeleteChangeEventsOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
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
func (m *MockStorage) GetServicePreferences(ctx context.Context, serviceName string) ([]string, error) {
	return []string{}, nil
}
func (m *MockStorage) SaveServicePreferences(ctx context.Context, serviceName string, visibleMetrics []string) error {
	return nil
}

func (m *MockStorage) GetAnalyzedEventIDs(ctx context.Context, eventIDs []string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (m *MockStorage) GetNamespaces(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

// MockMetrics implements metrics.MetricsProvider
type MockMetrics struct {
	baseline domain.MetricValues
	impact   domain.MetricValues
}

func (m *MockMetrics) GetAverageMetrics(ctx context.Context, services []string, start, end time.Time) (domain.MetricValues, error) {
	return m.baseline, nil
}

func (m *MockMetrics) GetAvailableMetrics() []domain.MetricInfo {
	return []domain.MetricInfo{}
}

// Better MockMetrics that allows specifying return values for specific calls
type ControllableMetrics struct {
	Calls            []domain.MetricValues
	Index            int
	AvailableMetrics []domain.MetricInfo
}

func (m *ControllableMetrics) GetAverageMetrics(ctx context.Context, services []string, start, end time.Time) (domain.MetricValues, error) {
	if m.Index >= len(m.Calls) {
		return domain.MetricValues{}, nil
	}
	val := m.Calls[m.Index]
	m.Index++
	return val, nil
}

func (m *ControllableMetrics) GetAvailableMetrics() []domain.MetricInfo {
	return m.AvailableMetrics
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
	cfg.Analysis.WeightsBuiltIn = map[string]float64{
		"error_rate":     0.4,
		"latency_p95_ms": 0.3,
	}
	cfg.Analysis.WeightsCustom = map[string]float64{}

	store := &MockStorage{}
	
	// Setup metrics: Baseline (good), Impact (bad errors AND latency)
	metrics := &ControllableMetrics{
		Calls: []domain.MetricValues{
			// Baseline (Non-zero errors to allow >100% delta)
			{ErrorRate: 0.01, LatencyP95: 100, RPS: 100}, 
			// Impact: 
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

	if analysis.ConfidenceScore != 0.5 {
		t.Errorf("expected confidence score 0.5 for low RPS (both baseline and impact < 1.0), got %f", analysis.ConfidenceScore)
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
	cfg.Analysis.WeightsBuiltIn = map[string]float64{
		"rps": 1.0, // Only consider RPS
	}
	cfg.Analysis.WeightsCustom = map[string]float64{}
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

	// RPS drop of 100% should be a normalized score of 1.0.
	// With only RPS weighted, the final score should be 1.0.
	if analysis.ImpactScore != 1.0 {
		t.Errorf("expected exactly 1.0 score for 100%% RPS drop with only RPS weighted, got %f", analysis.ImpactScore)
	}
}

func TestAnalyzeImpact_ZeroBaseline(t *testing.T) {
	cfg := &config.Config{}
	cfg.Analysis.WeightsBuiltIn = map[string]float64{
		"error_rate": 1.0, // Only consider error rate
	}
	cfg.Analysis.WeightsCustom = map[string]float64{}
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
	// With only error rate weighted, the final score is 0.5
	expectedScore := 0.5
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
func (m *ErrorMetrics) GetAvailableMetrics() []domain.MetricInfo {
	return []domain.MetricInfo{}
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
	metrics := &ControllableMetrics{Calls: []domain.MetricValues{{}, {}}, AvailableMetrics: []domain.MetricInfo{}}
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

func TestRankChanges_Empty(t *testing.T) {
	cfg := &config.Config{}
	store := &MockStorage{changeEvents: []domain.ChangeEvent{}}
	metrics := &ControllableMetrics{}
	engine := correlator.NewEngine(store, metrics, cfg)

	ranked, err := engine.RankChanges(context.Background(), "api", time.Now().Add(-1*time.Hour), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ranked) != 0 {
		t.Errorf("expected 0 ranked changes, got %d", len(ranked))
	}
}

func TestRankChanges_SingleEvent(t *testing.T) {
	cfg := &config.Config{}
	eventTime := time.Now().Add(-1 * time.Hour)

	store := &MockStorage{
		changeEvents: []domain.ChangeEvent{
			{
				ID:               "evt-1",
				ChangeType:       "deployment_rollout",
				Timestamp:        eventTime,
				AffectedServices: []string{"api"},
			},
		},
	}

	metrics := &ControllableMetrics{
		Calls: []domain.MetricValues{
			{RPS: 100, ErrorRate: 0.01},
			{RPS: 100, ErrorRate: 0.05},
		},
	}

	engine := correlator.NewEngine(store, metrics, cfg)
	ranked, err := engine.RankChanges(context.Background(), "api", eventTime.Add(-30*time.Minute), eventTime.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ranked) != 1 {
		t.Fatalf("expected 1 ranked change, got %d", len(ranked))
	}
	if ranked[0].Rank != 1 {
		t.Errorf("expected rank 1, got %d", ranked[0].Rank)
	}
	if ranked[0].LikelihoodScore <= 0 {
		t.Error("expected positive likelihood score")
	}
	if ranked[0].ChangeTypeWeight != 1.0 {
		t.Errorf("expected change_type_weight 1.0 for deployment_rollout, got %f", ranked[0].ChangeTypeWeight)
	}
	if ranked[0].ServiceScope != 1.0 {
		t.Errorf("expected service_scope 1.0 for direct service, got %f", ranked[0].ServiceScope)
	}
}

func TestRankChanges_SortsByLikelihood(t *testing.T) {
	cfg := &config.Config{}
	baseTime := time.Now().Add(-2 * time.Hour)

	store := &MockStorage{
		changeEvents: []domain.ChangeEvent{
			{
				ID:               "evt-config",
				ChangeType:       "configmap_update",
				Timestamp:        baseTime,
				AffectedServices: []string{"api"},
			},
			{
				ID:               "evt-deploy",
				ChangeType:       "deployment_rollout",
				Timestamp:        baseTime.Add(5 * time.Minute),
				AffectedServices: []string{"api"},
			},
		},
	}

	// Both events get the same metric results (high impact)
	metrics := &ControllableMetrics{
		Calls: []domain.MetricValues{
			{RPS: 100, ErrorRate: 0.01}, // baseline for evt-config
			{RPS: 50, ErrorRate: 0.10},  // impact for evt-config
			{RPS: 100, ErrorRate: 0.01}, // baseline for evt-deploy
			{RPS: 50, ErrorRate: 0.10},  // impact for evt-deploy
		},
	}

	engine := correlator.NewEngine(store, metrics, cfg)
	ranked, err := engine.RankChanges(context.Background(), "api", baseTime.Add(-10*time.Minute), baseTime.Add(20*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ranked) != 2 {
		t.Fatalf("expected 2 ranked changes, got %d", len(ranked))
	}

	// deployment_rollout has higher change_type_weight (1.0) than configmap_update (0.5)
	// With similar impact scores, deployment should rank higher
	if ranked[0].Rank != 1 || ranked[1].Rank != 2 {
		t.Error("expected sequential ranks 1, 2")
	}

	if ranked[0].LikelihoodScore < ranked[1].LikelihoodScore {
		t.Error("expected ranked[0] to have higher likelihood than ranked[1]")
	}
}

func TestRankChanges_IndirectServiceScope(t *testing.T) {
	cfg := &config.Config{}
	eventTime := time.Now().Add(-2 * time.Hour)

	store := &MockStorage{
		changeEvents: []domain.ChangeEvent{
			{
				ID:               "evt-indirect",
				ChangeType:       "deployment_rollout",
				Timestamp:        eventTime,
				AffectedServices: []string{"database"}, // not the queried service
			},
		},
	}

	metrics := &ControllableMetrics{
		Calls: []domain.MetricValues{
			{RPS: 100}, {RPS: 100},
		},
	}

	engine := correlator.NewEngine(store, metrics, cfg)
	ranked, err := engine.RankChanges(context.Background(), "api", eventTime.Add(-30*time.Minute), eventTime.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ranked) != 1 {
		t.Fatalf("expected 1 ranked change, got %d", len(ranked))
	}

	if ranked[0].ServiceScope != 0.5 {
		t.Errorf("expected service_scope 0.5 for indirect service, got %f", ranked[0].ServiceScope)
	}
}

func TestRankChanges_PendingEvent(t *testing.T) {
	cfg := &config.Config{}
	cfg.Analysis.ImpactDur = 30 * time.Minute

	// Event happened just now — impact window still open
	eventTime := time.Now()
	store := &MockStorage{
		changeEvents: []domain.ChangeEvent{
			{
				ID:               "evt-pending",
				ChangeType:       "deployment_rollout",
				Timestamp:        eventTime,
				AffectedServices: []string{"api"},
			},
		},
	}

	metrics := &ControllableMetrics{}
	engine := correlator.NewEngine(store, metrics, cfg)

	ranked, err := engine.RankChanges(context.Background(), "api", eventTime.Add(-10*time.Minute), eventTime.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ranked) != 1 {
		t.Fatalf("expected 1 ranked change (pending), got %d", len(ranked))
	}

	if ranked[0].Analysis.ImpactLevel != "PENDING" {
		t.Errorf("expected PENDING impact level, got %s", ranked[0].Analysis.ImpactLevel)
	}
}

func TestAnalyzeImpact_AdditionalMetrics(t *testing.T) {
	// Setup config with a custom metric set
	cfg := &config.Config{}
	cfg.Analysis.BaselineDur = 30 * time.Minute
	cfg.Analysis.ImpactDur = 30 * time.Minute
	cfg.Analysis.WeightsBuiltIn = map[string]float64{
		"error_rate": 0.5,
	}
	cfg.Analysis.WeightsCustom = map[string]float64{
		"custom_metric_a": 0.5,
		"custom_metric_b": 0.5,
	}

	cfg.Prometheus.AdditionalMetrics = []config.PrometheusMetric{
		{Name: "custom_metric_a", Query: "some_promql_query_a"},
		{Name: "custom_metric_b", Query: "some_promql_query_b"},
	}

	store := &MockStorage{}

	// Let's create two distinct AdditionalMetrics data for baseline and impact
	baselineAdditional := map[string]float64{
		"custom_metric_a": 10.0,
		"custom_metric_b": 20.0,
	}
	impactAdditional := map[string]float64{
		"custom_metric_a": 15.0, // Increased by 50% -> delta 0.5 -> norm 0.25
		"custom_metric_b": 10.0, // Decreased by 50% -> delta -0.5 -> norm 0
	}

	metricsWithAdditional := &ControllableMetrics{
		Calls: []domain.MetricValues{
			// Baseline call
			{ErrorRate: 0.01, LatencyP95: 100, RPS: 100, AdditionalMetrics: baselineAdditional},
			// Impact call
			{ErrorRate: 0.01, LatencyP95: 100, RPS: 100, AdditionalMetrics: impactAdditional},
		},
		AvailableMetrics: []domain.MetricInfo{{Name: "custom_metric_a"}, {Name: "custom_metric_b"}},
	}

	engine := correlator.NewEngine(store, metricsWithAdditional, cfg)

	event := domain.ChangeEvent{
		ID:        "evt-additional-metrics",
		Timestamp: time.Now().Add(-1 * time.Hour),
	}

	analysis, err := engine.AnalyzeImpact(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assertions for additional metrics
	if analysis.BaselineMetrics.AdditionalMetrics["custom_metric_a"] != 10.0 {
		t.Errorf("expected baseline custom_metric_a to be 10.0, got %f", analysis.BaselineMetrics.AdditionalMetrics["custom_metric_a"])
	}
	if analysis.ImpactMetrics.AdditionalMetrics["custom_metric_a"] != 15.0 {
		t.Errorf("expected impact custom_metric_a to be 15.0, got %f", analysis.ImpactMetrics.AdditionalMetrics["custom_metric_a"])
	}
	if analysis.Deltas.AdditionalMetrics["custom_metric_a"] != 0.5 { // (15-10)/10 = 0.5
		t.Errorf("expected delta custom_metric_a to be 0.5, got %f", analysis.Deltas.AdditionalMetrics["custom_metric_a"])
	}

	if analysis.Deltas.AdditionalMetrics["custom_metric_b"] != -0.5 { // (10-20)/20 = -0.5
		t.Errorf("expected delta custom_metric_b to be -0.5, got %f", analysis.Deltas.AdditionalMetrics["custom_metric_b"])
	}

	// Score calculation
	// error_rate delta is 0, score is 0.
	// custom_metric_a delta is 0.5, norm score is 0.25
	// custom_metric_b delta is -0.5, norm score is 0
	// weights: error_rate: 0.5, custom_metric_a: 0.5, custom_metric_b: 0.5
	// total weight = 0.5 + 0.5 + 0.5 = 1.5
	// score = (0 * (0.5/1.5)) + (0.25 * (0.5/1.5)) + (0 * (0.5/1.5))
	// score = 0.25 * (1/3) = 0.08333...
	expectedScore := 0.08333333333333333
	if analysis.ImpactScore < expectedScore-0.001 || analysis.ImpactScore > expectedScore+0.001 {
		t.Errorf("expected score around %f, got %f", expectedScore, analysis.ImpactScore)
	}

	// Also test the GetAvailableMetrics()
	availableMetrics := metricsWithAdditional.GetAvailableMetrics()
	if len(availableMetrics) != 2 {
		t.Errorf("expected 2 available metrics, got %d", len(availableMetrics))
	}
}
