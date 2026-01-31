package metrics

import (
	"bytes"
	"context"
	"fmt"
	"text/template"
	"time"
	"valiant/internal/domain"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type PrometheusClient struct {
	api     v1.API
	queries map[string]string
}

func NewPrometheusClient(apiURL string, queries map[string]string) (*PrometheusClient, error) {
	client, err := api.NewClient(api.Config{
		Address: apiURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus client: %w", err)
	}

	return &PrometheusClient{
		api:     v1.NewAPI(client),
		queries: queries,
	}, nil
}

func (p *PrometheusClient) GetAverageMetrics(ctx context.Context, services []string, start, end time.Time) (domain.MetricValues, error) {
	duration := end.Sub(start)
	durationStr := fmt.Sprintf("%dm", int(duration.Minutes()))

	data := struct {
		Services string
		Duration string
	}{
		Services: joinServices(services),
		Duration: durationStr,
	}

	var values domain.MetricValues

	// Helper to query a single value
	queryMetric := func(metricKey string) (float64, error) {
		queryTmpl, ok := p.queries[metricKey]
		if !ok {
			return 0, fmt.Errorf("query for %s not defined", metricKey)
		}

		tmpl, err := template.New(metricKey).Parse(queryTmpl)
		if err != nil {
			return 0, fmt.Errorf("failed to parse template: %w", err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return 0, fmt.Errorf("failed to execute template: %w", err)
		}

		query := buf.String()
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

	values.ErrorRate, _ = queryMetric("error_rate")
	latency, _ := queryMetric("latency_p95")
	values.LatencyP95 = latency * 1000 // Assume query returns seconds, convert to ms
	values.RPS, _ = queryMetric("rps")
	values.CPUSaturation, _ = queryMetric("cpu_saturation")
	values.MemorySaturation, _ = queryMetric("memory_saturation")

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
