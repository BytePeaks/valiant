package correlator_test

import (
	"context"
	"testing"
	"time"
	"valiant/internal/config"
	"valiant/internal/correlator"
	"valiant/internal/domain"
)

// LinkingMockStorage implements storage.Storage for linking tests
type LinkingMockStorage struct {
	// All possible events the mock can return
	availableChangeEvents []domain.ChangeEvent
	// Filters that were applied in the last call to GetChangeEvents
	filtersApplied map[string]interface{}
}

func (m *LinkingMockStorage) SaveChangeEvent(ctx context.Context, event domain.ChangeEvent) error { return nil }
func (m *LinkingMockStorage) GetChangeEvents(ctx context.Context, filters map[string]interface{}) ([]domain.ChangeEvent, int, error) {
	m.filtersApplied = filters
	var results []domain.ChangeEvent

	from, fromOk := filters["from_timestamp"].(time.Time)
	to, toOk := filters["to_timestamp"].(time.Time)

	for _, event := range m.availableChangeEvents {
		// Basic time filtering
		if fromOk && event.Timestamp.Before(from) {
			continue
		}
		if toOk && event.Timestamp.After(to) {
			continue
		}
		// Basic trigger type filtering
		if triggerType, ok := filters["trigger_type"]; ok && event.TriggerType != triggerType {
			continue
		}
		// Basic metadata filtering
		if metadataFilter, ok := filters["metadata_has_any"].(map[string]string); ok {
			match := false
			for key, value := range metadataFilter {
				if event.Metadata[key] == value {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		results = append(results, event)
	}

	return results, len(results), nil
}
func (m *LinkingMockStorage) DeleteChangeEventsOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}
func (m *LinkingMockStorage) GetChangeEventByID(ctx context.Context, id string) (domain.ChangeEvent, error) {
	return domain.ChangeEvent{}, nil
}
func (m *LinkingMockStorage) GetServices(ctx context.Context) ([]string, error) { return nil, nil }
func (m *LinkingMockStorage) GetEventsPendingAnalysis(ctx context.Context) ([]domain.ChangeEvent, error) {
	return nil, nil
}
func (m *LinkingMockStorage) SaveImpactAnalysis(ctx context.Context, analysis domain.ImpactAnalysis) error { return nil }
func (m *LinkingMockStorage) GetImpactAnalysisByEventID(ctx context.Context, eventID string) (*domain.ImpactAnalysis, error) {
	return nil, nil
}
func (m *LinkingMockStorage) GetServicePreferences(ctx context.Context, serviceName string) ([]string, error) {
	return []string{}, nil
}
func (m *LinkingMockStorage) SaveServicePreferences(ctx context.Context, serviceName string, visibleMetrics []string) error {
	return nil
}

func (m *LinkingMockStorage) GetAnalyzedEventIDs(ctx context.Context, eventIDs []string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (m *LinkingMockStorage) GetNamespaces(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func TestIntentExecutionLinking(t *testing.T) {
	cfg := &config.Config{}
	cfg.Analysis.IntentExecutionCorrelationDur = 1 * time.Hour
	metrics := &ControllableMetrics{Calls: []domain.MetricValues{{}, {}}} // Dummy metrics
	eventTime := time.Now().Add(-2 * time.Hour)

	baseExecutionEvent := domain.ChangeEvent{
		ID:          "exec-evt-1",
		TriggerType: "GitOps",
		Timestamp:   eventTime,
		Metadata:    map[string]string{"git_commit_sha": "abcdef123"},
	}

	t.Run("Successful Link via git_sha", func(t *testing.T) {
		store := &LinkingMockStorage{
			availableChangeEvents: []domain.ChangeEvent{
				{ID: "ci-evt-1", TriggerType: "CI", Timestamp: eventTime.Add(-10 * time.Minute), Metadata: map[string]string{"git_commit_sha": "abcdef123"}},
			},
		}
		engine := correlator.NewEngine(store, metrics, cfg)
		analysis, err := engine.AnalyzeImpact(context.Background(), baseExecutionEvent)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if analysis.IsOrphaned {
			t.Error("Expected IsOrphaned to be false, but it was true")
		}
		
		metadataFilter := store.filtersApplied["metadata_has_any"].(map[string]string)
		if metadataFilter["git_commit_sha"] != "abcdef123" {
			t.Errorf("Incorrect git_commit_sha in metadata filter")
		}
	})
	
	t.Run("Successful Link via image_tag", func(t *testing.T) {
		store := &LinkingMockStorage{
			availableChangeEvents: []domain.ChangeEvent{
				{ID: "ci-evt-2", TriggerType: "CI", Timestamp: eventTime.Add(-10 * time.Minute), Metadata: map[string]string{"image_tag": "v1.2.3"}},
			},
		}
		engine := correlator.NewEngine(store, metrics, cfg)
		event := domain.ChangeEvent{
			ID:          "exec-evt-2",
			TriggerType: "GitOps",
			Timestamp:   eventTime,
			Metadata:    map[string]string{"image_tag": "v1.2.3"},
		}
		analysis, err := engine.AnalyzeImpact(context.Background(), event)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if analysis.IsOrphaned {
			t.Error("Expected IsOrphaned to be false, but it was true")
		}
	})
	
	t.Run("Successful Link with both sha and tag", func(t *testing.T) {
		store := &LinkingMockStorage{
			availableChangeEvents: []domain.ChangeEvent{
				// This one matches the sha
				{ID: "ci-evt-sha", TriggerType: "CI", Timestamp: eventTime.Add(-10 * time.Minute), Metadata: map[string]string{"git_commit_sha": "abcdef123"}},
				// This one is irrelevant
				{ID: "ci-evt-other", TriggerType: "CI", Timestamp: eventTime.Add(-11 * time.Minute), Metadata: map[string]string{"git_commit_sha": "other"}},
			},
		}
		engine := correlator.NewEngine(store, metrics, cfg)
		event := domain.ChangeEvent{
			ID:          "exec-evt-3",
			TriggerType: "GitOps",
			Timestamp:   eventTime,
			Metadata:    map[string]string{"git_commit_sha": "abcdef123", "image_tag": "v1.2.3"},
		}
		analysis, err := engine.AnalyzeImpact(context.Background(), event)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if analysis.IsOrphaned {
			t.Error("Expected IsOrphaned to be false, but it was true")
		}
		// The mock should have found one event.
	})

	t.Run("Orphaned when no matching CI event exists", func(t *testing.T) {
		store := &LinkingMockStorage{
			availableChangeEvents: []domain.ChangeEvent{},
		}
		engine := correlator.NewEngine(store, metrics, cfg)
		analysis, err := engine.AnalyzeImpact(context.Background(), baseExecutionEvent)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if !analysis.IsOrphaned {
			t.Error("Expected IsOrphaned to be true, but it was false")
		}
	})
	
	t.Run("Successful link at the very edge of the time window", func(t *testing.T) {
		store := &LinkingMockStorage{
			availableChangeEvents: []domain.ChangeEvent{
				// This CI event is exactly at the boundary of the 1-hour window
				{ID: "ci-evt-edge", TriggerType: "CI", Timestamp: eventTime.Add(-1 * time.Hour), Metadata: map[string]string{"git_commit_sha": "abcdef123"}},
			},
		}
		engine := correlator.NewEngine(store, metrics, cfg)
		analysis, err := engine.AnalyzeImpact(context.Background(), baseExecutionEvent)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if analysis.IsOrphaned {
			t.Error("Expected IsOrphaned to be false when CI event is at the window edge, but it was true")
		}
	})
	
	t.Run("Orphaned when CI event is just outside the time window", func(t *testing.T) {
		store := &LinkingMockStorage{
			availableChangeEvents: []domain.ChangeEvent{
				// This CI event is 1 second outside the 1-hour window
				{ID: "ci-evt-outside", TriggerType: "CI", Timestamp: eventTime.Add(-1 * time.Hour).Add(-1 * time.Second), Metadata: map[string]string{"git_commit_sha": "abcdef123"}},
			},
		}
		engine := correlator.NewEngine(store, metrics, cfg)
		analysis, err := engine.AnalyzeImpact(context.Background(), baseExecutionEvent)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if !analysis.IsOrphaned {
			t.Error("Expected IsOrphaned to be true when CI event is outside the window, but it was false")
		}
	})

	t.Run("Orphaned when execution event has no linking metadata", func(t *testing.T) {
		store := &LinkingMockStorage{}
		engine := correlator.NewEngine(store, metrics, cfg)
		event := domain.ChangeEvent{
			ID:          "exec-evt-nometa",
			TriggerType: "GitOps",
			Timestamp:   eventTime,
			Metadata:    map[string]string{}, // No SHA or tag
		}
		analysis, err := engine.AnalyzeImpact(context.Background(), event)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if !analysis.IsOrphaned {
			t.Error("Expected IsOrphaned to be true for event with no linking metadata, but it was false")
		}
	})
	
	t.Run("Non-execution event is never orphaned", func(t *testing.T) {
		store := &LinkingMockStorage{}
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
			t.Error("Expected IsOrphaned to be false for a non-execution event, but it was true")
		}
	})

	t.Run("Multiple executions link to the same CI event", func(t *testing.T) {
		store := &LinkingMockStorage{
			availableChangeEvents: []domain.ChangeEvent{
				{ID: "ci-evt-multi", TriggerType: "CI", Timestamp: eventTime.Add(-10 * time.Minute), Metadata: map[string]string{"image_tag": "v3.0.0"}},
			},
		}
		engine := correlator.NewEngine(store, metrics, cfg)

		// First execution event
		execEvent1 := domain.ChangeEvent{
			ID:          "exec-evt-multi-1",
			TriggerType: "GitOps",
			Timestamp:   eventTime,
			Metadata:    map[string]string{"image_tag": "v3.0.0"},
		}
		analysis1, err := engine.AnalyzeImpact(context.Background(), execEvent1)
		if err != nil {
			t.Fatalf("Unexpected error on first event: %v", err)
		}
		if analysis1.IsOrphaned {
			t.Error("Expected first execution event not to be orphaned, but it was")
		}

		// Second execution event, slightly later
		execEvent2 := domain.ChangeEvent{
			ID:          "exec-evt-multi-2",
			TriggerType: "GitOps",
			Timestamp:   eventTime.Add(5 * time.Minute),
			Metadata:    map[string]string{"image_tag": "v3.0.0"},
		}
		analysis2, err := engine.AnalyzeImpact(context.Background(), execEvent2)
		if err != nil {
			t.Fatalf("Unexpected error on second event: %v", err)
		}
		if analysis2.IsOrphaned {
			t.Error("Expected second execution event not to be orphaned, but it was")
		}
	})

	t.Run("Orphaned when CI event is missing linking metadata", func(t *testing.T) {
		store := &LinkingMockStorage{
			availableChangeEvents: []domain.ChangeEvent{
				// This CI event corresponds in time, but has no metadata to link by
				{ID: "ci-evt-no-meta", TriggerType: "CI", Timestamp: eventTime.Add(-10 * time.Minute), Metadata: map[string]string{}},
			},
		}
		engine := correlator.NewEngine(store, metrics, cfg)
		analysis, err := engine.AnalyzeImpact(context.Background(), baseExecutionEvent)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !analysis.IsOrphaned {
			t.Error("Expected IsOrphaned to be true when CI event is missing metadata, but it was false")
		}
	})

	t.Run("Linkable manual event is not orphaned", func(t *testing.T) {
		store := &LinkingMockStorage{
			availableChangeEvents: []domain.ChangeEvent{
				{ID: "ci-evt-manual", TriggerType: "CI", Timestamp: eventTime.Add(-5 * time.Minute), Metadata: map[string]string{"git_commit_sha": "manual-sha"}},
			},
		}
		engine := correlator.NewEngine(store, metrics, cfg)
		manualEvent := domain.ChangeEvent{
			ID:          "manual-exec-1",
			TriggerType: "manual",
			Timestamp:   eventTime,
			Metadata:    map[string]string{"git_commit_sha": "manual-sha"},
		}
		analysis, err := engine.AnalyzeImpact(context.Background(), manualEvent)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if analysis.IsOrphaned {
			t.Error("Expected linkable manual event not to be orphaned, but it was")
		}
	})
}

