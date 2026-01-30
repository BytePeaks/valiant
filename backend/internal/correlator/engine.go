package correlator

import (
	"context"
	"valiant/internal/domain"
	"valiant/internal/metrics"
	"valiant/internal/storage"
)

type Engine struct {
	storage storage.Storage
	metrics metrics.MetricsProvider
}

func NewEngine(s storage.Storage, m metrics.MetricsProvider) *Engine {
	return &Engine{
		storage: s,
		metrics: m,
	}
}

func (e *Engine) AnalyzeImpact(ctx context.Context, event domain.ChangeEvent) (domain.ImpactAnalysis, error) {
	// 1. Define time windows
	// 2. Query metrics for windows
	// 3. Calculate impact score
	// 4. Return ImpactAnalysis
	return domain.ImpactAnalysis{ChangeEvent: event}, nil
}
