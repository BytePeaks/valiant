package shared

import (
	"testing"
	"valiant/internal/domain"
)

// AssertOrphaned validates orphan detection status
func AssertOrphaned(t *testing.T, analysis domain.ImpactAnalysis, expectedOrphaned bool) {
	t.Helper()
	if analysis.IsOrphaned != expectedOrphaned {
		t.Errorf("Orphan status mismatch:\n  Expected: %v\n  Actual:   %v",
			expectedOrphaned,
			analysis.IsOrphaned)
	}
}
