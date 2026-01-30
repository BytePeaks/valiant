package metrics

import (
	"context"
	"fmt"
	"time"
	"valiant/internal/domain"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type PrometheusClient struct {
	api v1.API
}

func NewPrometheusClient(apiURL string) (*PrometheusClient, error) {
	client, err := api.NewClient(api.Config{
		Address: apiURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus client: %w", err)
	}

	return &PrometheusClient{
		api: v1.NewAPI(client),
	}, nil
}

func (p *PrometheusClient) GetAverageMetrics(ctx context.Context, services []string, start, end time.Time) (domain.MetricValues, error) {
	duration := end.Sub(start)
	durationStr := fmt.Sprintf("%dm", int(duration.Minutes()))

	var values domain.MetricValues

	// Helper to query a single value
	queryAvg := func(queryTemplate string) (float64, error) {
		// This is a simplified aggregation over the service list.
		// In a real world, service names would be labels (e.g., {service="my-service"}).
		serviceFilter := ""
		if len(services) > 0 {
			serviceFilter = fmt.Sprintf(`{service=~"%s"}`, joinServices(services))
		}

		query := fmt.Sprintf(queryTemplate, serviceFilter, durationStr)
		val, _, err := p.api.Query(ctx, query, end)
		if err != nil {
			return 0, err
		}

		vector, ok := val.(model.Vector)
		if !ok || len(vector) == 0 {
			return 0, nil
		}

		return float64(vector[0].Value), nil
	}

	// 1. Error Rate: avg_over_time(rate(http_requests_total{status=~"5.."}[1m])[30m])
	errorRate, _ := queryAvg(`avg_over_time(sum(rate(http_requests_total%s{status=~"5.."}[1m]))[%s])`)
	values.ErrorRate = errorRate

	// 2. Latency P95: avg_over_time(histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket[1m])))[30m])
	latency, _ := queryAvg(`avg_over_time(histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket%s[1m])))[%s])`)
	values.LatencyP95 = latency * 1000 // Convert to ms

	// 3. RPS: avg_over_time(sum(rate(http_requests_total[1m]))[30m])
	rps, _ := queryAvg(`avg_over_time(sum(rate(http_requests_total%s[1m]))[%s])`)
	values.RPS = rps

	// 4. CPU Saturation
	cpu, _ := queryAvg(`avg_over_time(sum(rate(container_cpu_usage_seconds_total%s[1m]))[%s])`)
	values.CPUSaturation = cpu

	// 5. Memory Saturation
	mem, _ := queryAvg(`avg_over_time(sum(container_memory_usage_bytes%s)[%s])`)
	values.MemorySaturation = mem

	return values, nil
}

func joinServices(services []string) string {
	res := ""
	for i, s := range services {
		if i > 0 {
			res += "|"
		}
		res += s
	}
	return res
}
