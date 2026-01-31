package correlator_test

import (
	"context"
	"testing"
	"time"
	"valiant/internal/config"
	"valiant/internal/correlator"
	"valiant/internal/domain"
)

// We reuse MockStorage and MockMetrics from engine_test.go if they were in the same package,
// but they are in correlator_test. Since they are helper types, I will redefine a minimal 
// version or move them to a shared test_helpers file. For now, I'll redefine what's needed.

type WorkerMockStorage struct {
	MockStorage // Embed existing mock
	pending     []domain.ChangeEvent
	processed   int
}

func (m *WorkerMockStorage) GetEventsPendingAnalysis(ctx context.Context) ([]domain.ChangeEvent, error) {
	return m.pending, nil
}

func (m *WorkerMockStorage) SaveImpactAnalysis(ctx context.Context, a domain.ImpactAnalysis) error {
	m.processed++
	return nil
}

func TestWorker_RunBatch(t *testing.T) {
	cfg, _ := config.Load("")
	store := &WorkerMockStorage{
		pending: []domain.ChangeEvent{
			{ID: "evt-ready", Timestamp: time.Now().Add(-1 * time.Hour)},
			{ID: "evt-too-soon", Timestamp: time.Now()},
		},
	}
	
	// Metrics return something so analysis succeeds for the ready one
	metrics := &ControllableMetrics{
		Calls: []domain.MetricValues{{RPS: 10}, {RPS: 10}},
	}
	
	engine := correlator.NewEngine(store, metrics, cfg)
	worker := correlator.NewWorker(engine)

	// Run one batch manually
	worker.RunBatchPublic(context.Background())

	// Should have processed exactly 1 (the one from 1 hour ago)
	if store.processed != 1 {
		t.Errorf("expected 1 processed event, got %d", store.processed)
	}
}
