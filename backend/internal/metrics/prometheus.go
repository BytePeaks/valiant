package metrics

import (
	"context"
	"time"
	"valiant/internal/domain"
)

type PrometheusClient struct {
	apiURL string
}

func NewPrometheusClient(apiURL string) *PrometheusClient {
	return &PrometheusClient{apiURL: apiURL}
}

func (p *PrometheusClient) GetAverageMetrics(ctx context.Context, services []string, start, end time.Time) (domain.MetricValues, error) {
	// TODO: Implement actual Prometheus query
	return domain.MetricValues{}, nil
}
