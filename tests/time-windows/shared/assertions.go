package shared

import (
	"testing"
	"time"
)

// AssertTimeEqual validates that two time.Time values are approximately equal within a tolerance.
func AssertTimeEqual(t *testing.T, actual, expected time.Time, tolerance time.Duration, msgAndArgs ...interface{}) {
	t.Helper()
	diff := actual.Sub(expected)
	if diff < -tolerance || diff > tolerance {
		t.Errorf("Time mismatch:\n  Expected: %v (±%v)\n  Actual:   %v\n  Diff:     %v", expected, tolerance, actual, diff)
	}
}

// AssertDurationEqual validates that two time.Duration values are approximately equal within a tolerance.
func AssertDurationEqual(t *testing.T, actual, expected, tolerance time.Duration, msgAndArgs ...interface{}) {
	t.Helper()
	diff := actual - expected
	if diff < -tolerance || diff > tolerance {
		t.Errorf("Duration mismatch:\n  Expected: %v (±%v)\n  Actual:   %v\n  Diff:     %v", expected, tolerance, actual, diff)
	}
}
