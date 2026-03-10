package shared

import (
	"valiant/internal/domain"
	"valiant/tests/common"
)

// SampleChangeEvent creates a test change event
func SampleChangeEvent() domain.ChangeEvent {
	return common.SampleChangeEvent()
}

// SampleCIEvent creates a test CI (Intent) event
func SampleCIEvent() domain.ChangeEvent {
	return common.SampleCIEvent()
}

// SampleMetricValues creates test metric values
func SampleMetricValues(errorRate, latencyP95, rps, cpu, memory float64) domain.MetricValues {
	return common.SampleMetricValues(errorRate, latencyP95, rps, cpu, memory)
}

// NoImpactMetrics creates baseline metrics with no change
func NoImpactMetrics() (baseline, impact domain.MetricValues) {
	return common.NoImpactMetrics()
}

// LowImpactMetrics creates metrics with small error rate increase
func LowImpactMetrics() (baseline, impact domain.MetricValues) {
	return common.LowImpactMetrics()
}

// MediumImpactMetrics creates metrics with moderate degradation
func MediumImpactMetrics() (baseline, impact domain.MetricValues) {
	return common.MediumImpactMetrics()
}

// HighImpactMetrics creates metrics with severe degradation
func HighImpactMetrics() (baseline, impact domain.MetricValues) {
	return common.HighImpactMetrics()
}
