package metrics

import (
	"bytes"
	"context"
	"fmt"
	"text/template"
	"time"
	"valiant/internal/config" // Import config package
	"valiant/internal/domain"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type PrometheusClient struct {
	api     v1.API
	queries map[string]string
	additionalMetrics []config.PrometheusMetric
	config  *config.Config // Add config reference
}

// NewPrometheusClient creates a new Prometheus client.
func NewPrometheusClient(apiURL string, queries map[string]string, additionalMetrics []config.PrometheusMetric, cfg *config.Config) (*PrometheusClient, error) {
	client, err := api.NewClient(api.Config{
		Address: apiURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus client: %w", err)
	}

	return &PrometheusClient{
		api:     v1.NewAPI(client),
		queries: queries,
		additionalMetrics: additionalMetrics,
		config:  cfg, // Store config
	}, nil
}

// QueryMetric fetches a single metric's value from Prometheus.
func (p *PrometheusClient) QueryMetric(ctx context.Context, metricName string, services []string, end time.Time, duration time.Duration) (float64, error) {
	var queryTmpl string
	var ok bool

	queryTmpl, ok = p.queries[metricName]
	if !ok {
		for _, m := range p.additionalMetrics {
			if m.Name == metricName {
				queryTmpl = m.Query
				ok = true
				break
			}
		}
	}

	if !ok {
		return 0, fmt.Errorf("query for metric '%s' not defined", metricName)
	}

	data := struct {
		Services string
		Duration string
	}{
		Services: joinServices(services),
		Duration: fmt.Sprintf("%dm", int(duration.Minutes())),
	}

	tmpl, err := template.New(metricName).Parse(queryTmpl)
	if err != nil {
		return 0, fmt.Errorf("failed to parse template for metric '%s': %w", metricName, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return 0, fmt.Errorf("failed to execute template for metric '%s': %w", metricName, err)
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

func (p *PrometheusClient) GetAvailableMetrics() []domain.MetricInfo {
	metrics := make([]domain.MetricInfo, 0, len(p.queries)+len(p.additionalMetrics))

	// Collect all weights to normalize them later
	allWeights := make(map[string]float64)
	totalWeight := 0.0

	// Add built-in metric weights
	for name, weight := range p.config.Analysis.WeightsBuiltIn {
		allWeights[name] = weight
		totalWeight += weight
	}
	// Add custom metric weights
	for name, weight := range p.config.Analysis.WeightsCustom {
		allWeights[name] = weight
		totalWeight += weight
	}

	// Normalize weights
	normalizedWeights := make(map[string]float64)
	if totalWeight > 0 {
		for name, weight := range allWeights {
			normalizedWeights[name] = weight / totalWeight
		}
	}


	for name, queryTemplate := range p.queries {
		var icon string
		switch name {
		case "error_rate":
			icon = "AlertCircle"
		case "latency_p95_ms":
			icon = "Clock"
		case "rps":
			icon = "Zap"
		case "cpu":
			icon = "Cpu"
		case "memory":
			icon = "Database"
		default:
			icon = "Activity"
		}
		metrics = append(metrics, domain.MetricInfo{
			Name: name,
			Icon: icon,
			Weight: normalizedWeights[name],
			Type: "builtin", // Mark as built-in
			Promql: queryTemplate, // This holds the template for built-in queries
		})
	}
	for _, metric := range p.additionalMetrics {
		metrics = append(metrics, domain.MetricInfo{
			Name: metric.Name,
			Icon: metric.Icon,
			Weight: normalizedWeights[metric.Name],
			Type: "custom", // Mark as custom
			Query: metric.Query, // This holds the actual query for custom metrics
		})
	}
	return metrics
}
		
		
		func (p *PrometheusClient) GetAverageMetrics(ctx context.Context, services []string, start, end time.Time) (domain.MetricValues, error) {
			duration := end.Sub(start)
		
			var values domain.MetricValues
			values.AdditionalMetrics = make(map[string]float64)
		
			// Fetch core metrics
			for metricKey := range p.queries {
				val, err := p.QueryMetric(ctx, metricKey, services, end, duration)
				if err != nil {
					fmt.Printf("Error querying core metric %s: %v\n", metricKey, err)
					continue
				}
				// This part needs to be smarter
				switch metricKey {
				case "error_rate":
					values.ErrorRate = val
				case "latency_p95_ms":
					values.LatencyP95 = val * 1000 // Convert to ms
				case "rps":
					values.RPS = val
				case "cpu":
					values.CPU = val
				case "memory":
					values.Memory = val
				}
			}
		

	// Fetch additional metrics
	for _, metric := range p.additionalMetrics {
		val, err := p.QueryMetric(ctx, metric.Name, services, end, duration)
		if err != nil {
			fmt.Printf("Error querying additional metric %s: %v\n", metric.Name, err)
			continue
		}
		values.AdditionalMetrics[metric.Name] = val
	}

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
