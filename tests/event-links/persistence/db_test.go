package persistence

import (
	"context"
	"testing"
	"time"
	"valiant/internal/domain"
	"valiant/internal/storage"
	"valiant/tests/common"
	"valiant/tests/event-links/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndGetEventLinks(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	store := storage.NewPostgresStorage(db, cfg)

	// Save two events first (foreign key references)
	ciEvent := common.SampleCIEvent()
	execEvent := common.SampleChangeEvent()

	err = store.SaveChangeEvent(context.Background(), ciEvent)
	require.NoError(t, err)
	err = store.SaveChangeEvent(context.Background(), execEvent)
	require.NoError(t, err)

	link := domain.EventLink{
		ID:               "link-test-1",
		IntentEventID:    ciEvent.ID,
		ExecutionEventID: execEvent.ID,
		LinkType:         "sha_match",
		Confidence:       1.0,
		CreatedAt:        time.Now().UTC(),
		Metadata:         map[string]string{"git_commit_sha": "abc123"},
	}

	// ACT
	err = store.SaveEventLink(context.Background(), link)
	require.NoError(t, err)

	// Get by event ID (intent side)
	links, err := store.GetEventLinksByEventID(context.Background(), ciEvent.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, "sha_match", links[0].LinkType)
	assert.Equal(t, 1.0, links[0].Confidence)

	// Get by event ID (execution side)
	links, err = store.GetEventLinksByEventID(context.Background(), execEvent.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, "sha_match", links[0].LinkType)
}

func TestUpsertDeduplication(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	store := storage.NewPostgresStorage(db, cfg)

	ciEvent := common.SampleCIEvent()
	execEvent := common.SampleChangeEvent()

	err = store.SaveChangeEvent(context.Background(), ciEvent)
	require.NoError(t, err)
	err = store.SaveChangeEvent(context.Background(), execEvent)
	require.NoError(t, err)

	link := domain.EventLink{
		ID:               "link-dup-1",
		IntentEventID:    ciEvent.ID,
		ExecutionEventID: execEvent.ID,
		LinkType:         "sha_match",
		Confidence:       1.0,
		CreatedAt:        time.Now().UTC(),
	}

	// ACT — save same link twice
	err = store.SaveEventLink(context.Background(), link)
	require.NoError(t, err)

	link.ID = "link-dup-2" // Different ID but same unique constraint
	err = store.SaveEventLink(context.Background(), link)
	require.NoError(t, err)

	// ASSERT — should only have one link
	links, err := store.GetEventLinksByEventID(context.Background(), ciEvent.ID)
	require.NoError(t, err)
	assert.Len(t, links, 1, "Upsert should deduplicate by (intent, execution, link_type)")
}

func TestCascadeDeleteOnEventRemoval(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	store := storage.NewPostgresStorage(db, cfg)

	ciEvent := common.SampleCIEvent()
	ciEvent.Timestamp = time.Now().Add(-100 * time.Hour) // Old event for deletion

	execEvent := common.SampleChangeEvent()
	execEvent.Timestamp = time.Now().Add(-100 * time.Hour) // Old event

	err = store.SaveChangeEvent(context.Background(), ciEvent)
	require.NoError(t, err)
	err = store.SaveChangeEvent(context.Background(), execEvent)
	require.NoError(t, err)

	link := domain.EventLink{
		ID:               "link-cascade-1",
		IntentEventID:    ciEvent.ID,
		ExecutionEventID: execEvent.ID,
		LinkType:         "sha_match",
		Confidence:       1.0,
		CreatedAt:        time.Now().UTC(),
	}

	err = store.SaveEventLink(context.Background(), link)
	require.NoError(t, err)

	// Verify link exists
	links, err := store.GetEventLinksByEventID(context.Background(), ciEvent.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)

	// ACT — delete old events
	cutoff := time.Now().Add(-50 * time.Hour) // This should delete events older than 50h
	_, err = store.DeleteChangeEventsOlderThan(context.Background(), cutoff)
	require.NoError(t, err)

	// ASSERT — links should be cascade deleted
	links, err = store.GetEventLinksByEventID(context.Background(), ciEvent.ID)
	require.NoError(t, err)
	assert.Empty(t, links, "Links should be cascade deleted with the event")
}
