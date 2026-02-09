package common

import (
	"time"
	"valiant/internal/config"
	"valiant/internal/domain"
)

// SampleChangeEvent creates a test change event
func SampleChangeEvent() domain.ChangeEvent {
	endTime := time.Now().Add(-30 * time.Minute)
	return domain.ChangeEvent{
		ID:               "test-event-1",
		Source:           "kubernetes", // Deprecated but still in use
		TriggerType:      "GitOps",
		ExecutionID:      "deploy-12345",
		ChangeType:       "deployment_rollout",
		Timestamp:        time.Now().Add(-1 * time.Hour),
		EndTime:          &endTime,
		AffectedServices: []string{"api-service"},
		Metadata: map[string]string{
			"git_commit_sha": "abc123def456",
			"image_tag":      "v1.2.3",
			"namespace":      "production",
		},
		Summary: "Deployed api-service v1.2.3 to production",
	}
}

// SampleCIEvent creates a test CI (Intent) event
func SampleCIEvent() domain.ChangeEvent {
	return domain.ChangeEvent{
		ID:               "test-ci-1",
		Source:           "ci-cd",
		TriggerType:      "CI",
		ExecutionID:      "build-789",
		ChangeType:       "build_success",
		Timestamp:        time.Now().Add(-2 * time.Hour),
		AffectedServices: []string{"api-service"},
		Metadata: map[string]string{
			"git_commit_sha": "abc123def456",
			"branch":         "main",
			"pipeline_id":    "pipeline-123",
		},
		Summary: "CI build succeeded for api-service",
	}
}



// SampleConfig returns a fully populated config.Config object with sensible defaults for testing purposes.
func SampleConfig() *config.Config {
	cfg := &config.Config{
		DatabaseURL: "postgres://user:password@localhost:5432/valiant_test?sslmode=disable",
		Port:        "8080",
		Prometheus: config.PrometheusConfig{
			URL: "http://localhost:9090",
			Queries: map[string]string{
				"error_rate":     `avg_over_time(sum(rate(http_requests_total{service=~"{{ .Services }}",status=~"5.."}[1m]))[{{ .Duration }}])`,
				"latency_p95_ms": `avg_over_time(histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket{service=~"{{ .Services }}"}[1m])))[{{ .Duration }}])`,
				"rps":            `avg_over_time(sum(rate(http_requests_total{service=~"{{ .Services }}"}[1m]))[{{ .Duration }}])`,
				"cpu":            `avg_over_time(sum(rate(container_cpu_usage_seconds_total{container=~"{{ .Services }}"}[1m]))[{{ .Duration }}])`,
				"memory":         `avg_over_over(sum(container_memory_usage_bytes{container=~"{{ .Services }}"})[{{ .Duration }}])`,
			},
		},
	}
	cfg.Retention.EventTTL = "90d"
	cfg.Retention.CleanupInterval = "1h"
	cfg.Analysis.BaselineWindow = "30m"
	cfg.Analysis.ImpactWindow = "30m"
	cfg.Analysis.IntentExecutionCorrelationWindow = "1h"
	cfg.Analysis.WeightsBuiltIn = map[string]float64{
		"error_rate":     0.4,
		"latency_p95_ms": 0.3,
		"cpu":            0.1,
		"memory":         0.1,
		"rps":            0.1,
	}
	cfg.Analysis.WeightsCustom = map[string]float64{}
	cfg.Worker.PollingInterval = "5m" // Default worker polling interval

	// Parse durations - these would normally be done by config.Load, but we do it manually for test config
	cfg.Analysis.BaselineDur, _ = time.ParseDuration(cfg.Analysis.BaselineWindow)
	cfg.Analysis.ImpactDur, _ = time.ParseDuration(cfg.Analysis.ImpactWindow)
	cfg.Analysis.IntentExecutionCorrelationDur, _ = time.ParseDuration(cfg.Analysis.IntentExecutionCorrelationWindow)
	cfg.Retention.EventTTLDur, _ = time.ParseDuration(cfg.Retention.EventTTL) // Simplified for test, assumes valid
	cfg.Retention.CleanupIntervalDur, _ = time.ParseDuration(cfg.Retention.CleanupInterval)
	cfg.Worker.PollingIntervalDur, _ = time.ParseDuration(cfg.Worker.PollingInterval)

	return cfg
}

// SampleMetricValues creates test metric values
func SampleMetricValues(errorRate, latencyP95, rps, cpu, memory float64) domain.MetricValues {
	return domain.MetricValues{
		ErrorRate:  errorRate,
		LatencyP95: latencyP95,
		RPS:        rps,
		CPU:        cpu,
		Memory:     memory,
	}
}

// NoImpactMetrics creates baseline metrics with no change
func NoImpactMetrics() (baseline, impact domain.MetricValues) {
	baseline = SampleMetricValues(0.01, 100, 100, 50, 60)
	impact = SampleMetricValues(0.01, 100, 100, 50, 60)
	return
}

// LowImpactMetrics creates metrics with small error rate increase
func LowImpactMetrics() (baseline, impact domain.MetricValues) {
	baseline = SampleMetricValues(0.01, 100, 100, 50, 60)
	impact = SampleMetricValues(0.03, 110, 95, 52, 62) // 2% error increase
	return
}

// MediumImpactMetrics creates metrics with moderate degradation
func MediumImpactMetrics() (baseline, impact domain.MetricValues) {
	baseline = SampleMetricValues(0.01, 100, 100, 50, 60)
	impact = SampleMetricValues(0.10, 200, 80, 70, 75) // 9% error increase + latency
	return
}

// HighImpactMetrics creates metrics with severe degradation
func HighImpactMetrics() (baseline, impact domain.MetricValues) {
	baseline = SampleMetricValues(0.01, 100, 100, 50, 60)
	impact = SampleMetricValues(0.50, 500, 50, 90, 85) // 49% error increase + severe latency
	return
}

// SampleConfigWithLinking creates a config with pre-defined linking templates for testing.
func SampleConfigWithLinking() *config.Config {
	return &config.Config{
		Linking: []config.LinkTemplate{
			{
				Name:        "View Commit",
				MetadataHas: []string{"git_commit_sha", "repository_url"},
				URLTemplate: "{{.repository_url}}/commit/{{.git_commit_sha}}",
			},
			{
				Name:        "View Jenkins Build",
				MetadataHas: []string{"jenkins_url", "jenkins_job_name", "jenkins_build_id"},
				URLTemplate: "{{.jenkins_url}}/job/{{.jenkins_job_name}}/{{.jenkins_build_id}}",
			},
			{
				Name:        "Open ArgoCD App",
				MetadataHas: []string{"argocd_url", "argocd_app_name"},
				URLTemplate: "{{.argocd_url}}/applications/{{.argocd_app_name}}",
			},
		},
	}
}
