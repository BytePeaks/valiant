package shared

import (
	"context"
	"time"
	"valiant/internal/domain"
)

// MetricCall records the parameters of a GetAverageMetrics call.
type MetricCall struct {
	Services []string
	Start    time.Time
	End      time.Time
}

// MockMetricsProvider implements metrics.MetricsProvider for testing
type MockMetricsProvider struct {
	Calls []MetricCall
}

func (m *MockMetricsProvider) GetAverageMetrics(ctx context.Context, services []string, start, end time.Time) (domain.MetricValues, error) {
	m.Calls = append(m.Calls, MetricCall{Services: services, Start: start, End: end})
	return domain.MetricValues{}, nil
}

func (m *MockMetricsProvider) GetAvailableMetrics() []domain.MetricInfo {
	return []domain.MetricInfo{}
}
