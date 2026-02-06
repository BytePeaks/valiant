package shared

import (
	"testing"
	"valiant/internal/domain"

	"github.com/stretchr/testify/assert"
)

// AssertChangeEvent asserts that a domain.ChangeEvent matches expected values.
// If expectedID is empty, it asserts that the actual ID is non-empty instead of checking equality.
func AssertChangeEvent(t *testing.T, actual domain.ChangeEvent, expectedID, expectedSource, expectedChangeType, expectedSummary string) {
	t.Helper()
	if expectedID == "" {
		assert.NotEmpty(t, actual.ID, "ChangeEvent ID should not be empty")
	} else {
		assert.Equal(t, expectedID, actual.ID, "ChangeEvent ID mismatch")
	}
	assert.Equal(t, expectedSource, actual.Source, "ChangeEvent Source mismatch")
	assert.Equal(t, expectedChangeType, actual.ChangeType, "ChangeEvent ChangeType mismatch")
	assert.Contains(t, actual.Summary, expectedSummary, "ChangeEvent Summary mismatch")
}

// AssertChangeEventMetadata asserts specific metadata key-value pairs in a ChangeEvent.
func AssertChangeEventMetadata(t *testing.T, actual domain.ChangeEvent, key, value string) {
	t.Helper()
	assert.Contains(t, actual.Metadata, key, "ChangeEvent metadata missing key %s", key)
	assert.Equal(t, value, actual.Metadata[key], "ChangeEvent metadata value mismatch for key %s", key)
}

// AssertChangeEventAffectedServices asserts that a ChangeEvent affects specific services.
func AssertChangeEventAffectedServices(t *testing.T, actual domain.ChangeEvent, expectedServices []string) {
	t.Helper()
	assert.ElementsMatch(t, expectedServices, actual.AffectedServices, "ChangeEvent AffectedServices mismatch")
}