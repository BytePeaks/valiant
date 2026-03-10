package shared

import (
	"testing"
	"valiant/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func AssertLinkExists(t *testing.T, links []domain.EventLink, linkType string, expectedConfidence float64, tolerance float64) {
	t.Helper()
	for _, link := range links {
		if link.LinkType == linkType {
			assert.InDelta(t, expectedConfidence, link.Confidence, tolerance,
				"Link confidence mismatch for type %s", linkType)
			return
		}
	}
	t.Errorf("Expected link of type %q not found in %d links", linkType, len(links))
}

func AssertNoLinkOfType(t *testing.T, links []domain.EventLink, linkType string) {
	t.Helper()
	for _, link := range links {
		if link.LinkType == linkType {
			t.Errorf("Unexpected link of type %q found: %+v", linkType, link)
			return
		}
	}
}

func AssertOrphaned(t *testing.T, analysis domain.ImpactAnalysis, expectedOrphaned bool) {
	t.Helper()
	require.Equal(t, expectedOrphaned, analysis.IsOrphaned, "Orphan status mismatch")
}
