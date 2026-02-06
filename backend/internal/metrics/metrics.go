package metrics

import (
	"context"
	"time"
	"valiant/internal/domain"
)

// MetricsProvider defines the interface for querying metrics from a source like Prometheus.
type MetricsProvider interface {
	GetAverageMetrics(ctx context.Context, services []string, start, end time.Time) (domain.MetricValues, error)
	GetAvailableMetrics() []domain.MetricInfo
}
