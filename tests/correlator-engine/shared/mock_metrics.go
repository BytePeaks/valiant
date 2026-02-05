package shared

import (
	"context"
	"time"
	"valiant/internal/domain"
)

// MockMetricsProvider implements metrics.MetricsProvider for testing
type MockMetricsProvider struct {
	// Map of service name + time range -> MetricValues
	mockData map[string]domain.MetricValues
}

// NewMockMetricsProvider creates a new mock metrics provider
func NewMockMetricsProvider() *MockMetricsProvider {
	return &MockMetricsProvider{
		mockData: make(map[string]domain.MetricValues),
	}
}

// SetMetrics configures the mock to return specific metrics for a service and time range
func (m *MockMetricsProvider) SetMetrics(service string, start, end time.Time, metrics domain.MetricValues) {
	key := m.makeKey(service, start, end)
	m.mockData[key] = metrics
}

// GetAverageMetrics returns mock metrics (implements metrics.MetricsProvider interface)
func (m *MockMetricsProvider) GetAverageMetrics(ctx context.Context, services []string, start, end time.Time) (domain.MetricValues, error) {
	// Simple mock: just use the first service
	if len(services) == 0 {
		return domain.MetricValues{}, nil
	}
	key := m.makeKey(services[0], start, end)
	if metrics, ok := m.mockData[key]; ok {
		return metrics, nil
	}
	// Return zero metrics if not configured
	return domain.MetricValues{}, nil
}

func (m *MockMetricsProvider) GetAvailableMetrics() []domain.MetricInfo {
	return []domain.MetricInfo{
		{Name: "error_rate", Icon: "AlertCircle"},
		{Name: "latency_p95_ms", Icon: "Clock"},
		{Name: "rps", Icon: "Zap"},
		{Name: "cpu", Icon: "Cpu"},
		{Name: "memory", Icon: "Database"},
	}
}

func (m *MockMetricsProvider) makeKey(service string, start, end time.Time) string {
	return service + "|" + start.Format(time.RFC3339) + "|" + end.Format(time.RFC3339)
}
