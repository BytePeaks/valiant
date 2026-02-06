package common

import (
	"time"
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
