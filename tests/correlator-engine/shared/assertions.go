package shared

import (
	"math"
	"testing"
	"valiant/internal/domain"
)

// AssertImpactScore validates impact score within tolerance
func AssertImpactScore(t *testing.T, analysis domain.ImpactAnalysis, expectedScore float64, tolerance float64) {
	t.Helper()
	actualScore := analysis.ImpactScore
	if math.Abs(actualScore-expectedScore) > tolerance {
		t.Errorf("Impact score mismatch:\n  Expected: %f ±%f\n  Actual:   %f\n  Delta:    %f",
			expectedScore, tolerance, actualScore, math.Abs(actualScore-expectedScore))
	}
}

// AssertImpactLevel validates impact level classification
func AssertImpactLevel(t *testing.T, analysis domain.ImpactAnalysis, expectedLevel string) {
	t.Helper()
	if analysis.ImpactLevel != expectedLevel {
		t.Errorf("Impact level mismatch:\n  Expected: %s\n  Actual:   %s",
			expectedLevel, analysis.ImpactLevel)
	}
}

// AssertOrphaned validates orphan detection status
func AssertOrphaned(t *testing.T, analysis domain.ImpactAnalysis, expectedOrphaned bool) {
	t.Helper()
	if analysis.IsOrphaned != expectedOrphaned {
		t.Errorf("Orphan status mismatch:\n  Expected: %v\n  Actual:   %v",
			expectedOrphaned, analysis.IsOrphaned)
	}
}

// AssertMetricDelta validates a specific metric delta
func AssertMetricDelta(t *testing.T, deltas domain.MetricValues, metricName string, expectedDelta float64, tolerance float64) {
	t.Helper()
	var actualDelta float64
	switch metricName {
	case "error_rate":
		actualDelta = deltas.ErrorRate
	case "latency_p95":
		actualDelta = deltas.LatencyP95
	case "rps":
		actualDelta = deltas.RPS
	case "cpu":
		actualDelta = deltas.CPU
	case "memory":
		actualDelta = deltas.Memory
	default:
		t.Fatalf("Unknown metric name: %s", metricName)
	}

	if math.Abs(actualDelta-expectedDelta) > tolerance {
		t.Errorf("Metric delta mismatch for %s:\n  Expected: %f ±%f\n  Actual:   %f",
			metricName, expectedDelta, tolerance, actualDelta)
	}
}

// AssertSnapshotExists validates that a snapshot was created
func AssertSnapshotExists(t *testing.T, analysis domain.ImpactAnalysis) {
	t.Helper()
	// Check that key fields are populated (indicates snapshot was retrieved)
	if analysis.ImpactScore == 0 && analysis.ImpactLevel == "" {
		t.Error("Snapshot appears empty (score=0, level='')")
	}
}

// AssertConfidenceScore validates confidence score
func AssertConfidenceScore(t *testing.T, analysis domain.ImpactAnalysis, expectedConfidence float64, tolerance float64) {
	t.Helper()
	if math.Abs(analysis.ConfidenceScore-expectedConfidence) > tolerance {
		t.Errorf("Confidence score mismatch:\n  Expected: %f ±%f\n  Actual:   %f",
			expectedConfidence, tolerance, analysis.ConfidenceScore)
	}
}
