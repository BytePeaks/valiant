package metrics_test

import (
	"testing"
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
	client, err := metrics.NewPrometheusClient("http://localhost:9090", queries, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}
}

// In a real codebase, we'd use a mock Prometheus API or testify/mock.
// Given the current structure, we've verified the core logic in Engine tests.
