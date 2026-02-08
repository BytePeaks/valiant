package metrics_test

import (
	"testing"
	"valiant/internal/config"
	"valiant/internal/metrics"
)

// Since we can't easily mock the Prometheus V1 API without a complex implementation,
// we will focus on testing the NewPrometheusClient initialization and potentially
// exporting internal methods if we want to test query generation logic.
// For now, we verify it initializes correctly.

func TestNewPrometheusClient(t *testing.T) {
	queries := map[string]string{
		"error_rate": "sum(rate(errors[{{ .Duration }}]))",
	}
	// Create a minimal config object for the test
	cfg := &config.Config{
		Prometheus: config.PrometheusConfig{ // Use the actual named type
			URL:     "http://localhost:9090",
			Queries: queries,
		},
		Analysis: config.AnalysisConfig{ // Use the actual named type
			WeightsBuiltIn: map[string]float64{"error_rate": 1.0},
			WeightsCustom:  map[string]float64{},
		},
	}

	client, err := metrics.NewPrometheusClient("http://localhost:9090", queries, nil, cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}
}

// In a real codebase, we'd use a mock Prometheus API or testify/mock.
// Given the current structure, we've verified the core logic in Engine tests.
