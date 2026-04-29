package metrics

import (
	"context"
	"errors"
	"time"
	"valiant/internal/domain"
)

// ErrPrometheusUnavailable is returned by UnavailableMetricsProvider when no Prometheus endpoint is configured.
var ErrPrometheusUnavailable = errors.New("Prometheus not configured")

// MetricsProvider defines the interface for querying metrics from a source like Prometheus.
type MetricsProvider interface {
	GetAverageMetrics(ctx context.Context, services []string, start, end time.Time) (domain.MetricValues, error)
	GetAvailableMetrics() []domain.MetricInfo
}

// UnavailableMetricsProvider satisfies MetricsProvider but returns ErrPrometheusUnavailable on every call.
// Used as a sentinel when no Prometheus endpoint was found at startup.
type UnavailableMetricsProvider struct{}

func (u *UnavailableMetricsProvider) GetAverageMetrics(_ context.Context, _ []string, _, _ time.Time) (domain.MetricValues, error) {
	return domain.MetricValues{}, ErrPrometheusUnavailable
}

func (u *UnavailableMetricsProvider) GetAvailableMetrics() []domain.MetricInfo {
	return nil
}
