package shared

import (
	"context"
	"time"
	"valiant/internal/domain"
)

type MockMetricsProvider struct{}

func (m *MockMetricsProvider) GetAverageMetrics(ctx context.Context, services []string, start, end time.Time) (domain.MetricValues, error) {
	return domain.MetricValues{}, nil
}

func (m *MockMetricsProvider) GetAvailableMetrics() []domain.MetricInfo {
	return []domain.MetricInfo{}
}
