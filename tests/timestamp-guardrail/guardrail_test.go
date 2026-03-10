package guardrail_test

import (
	"context"
	"testing"
	"time"
	"valiant/internal/config"
	"valiant/internal/domain"
	"valiant/internal/storage"
	"valiant/tests/timestamp-guardrail/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimestampGuardrail(t *testing.T) {
	// 1. Setup Test DB
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err, "Failed to set up test database")
	defer shared.CleanupTestDB(db, schemaName)

	// Use a nil config for this test as linking is not being tested
	// Pass an empty config as NewPostgresStorage now requires it
	store := storage.NewPostgresStorage(db, &config.Config{})
	ctx := context.Background()

	// 2. Define Test Events
	now := time.Now()
	// max_future_skew is 2 minutes, so 3 minutes is definitely in the future beyond tolerance
	futureTime := now.Add(3 * time.Minute)
	pastTime := now.Add(-1 * time.Minute)

	validEvent := domain.ChangeEvent{
		ID:        "valid-event-01",
		Source:    "test-source",
		Timestamp: pastTime,
		Summary:   "A valid event.",
	}

	futureTimestampEvent := domain.ChangeEvent{
		ID:        "invalid-event-future-timestamp",
		Source:    "test-source",
		Timestamp: futureTime,
		Summary:   "An event with a future timestamp.",
	}

	futureEndTime := now.Add(3 * time.Minute)
	futureEndTimeEvent := domain.ChangeEvent{
		ID:        "invalid-event-future-endtime",
		Source:    "test-source",
		Timestamp: pastTime,
		EndTime:   &futureEndTime,
		Summary:   "An event with a future end time.",
	}

	// 3. Save events
	err = store.SaveChangeEvent(ctx, validEvent)
	require.NoError(t, err)

	err = store.SaveChangeEvent(ctx, futureTimestampEvent)
	require.NoError(t, err)

	err = store.SaveChangeEvent(ctx, futureEndTimeEvent)
	require.NoError(t, err)

	// 4. Assertions for Saved Events
	t.Run("Verify saved events have correct status", func(t *testing.T) {
		// Check valid event
		savedValid, err := store.GetChangeEventByID(ctx, validEvent.ID)
		require.NoError(t, err)
		assert.Equal(t, "ready", savedValid.Status)
		assert.Empty(t, savedValid.InvalidReason)
		assert.Zero(t, savedValid.SkewSeconds)

		// Check event with future timestamp
		savedInvalidTs, err := store.GetChangeEventByID(ctx, futureTimestampEvent.ID)
		require.NoError(t, err)
		assert.Equal(t, "invalid_time", savedInvalidTs.Status)
		assert.Equal(t, "timestamp_in_future", savedInvalidTs.InvalidReason)
		// max_future_skew is 120s (2 minutes), so a timestamp 3 minutes (180s) in future
		// will result in skew_seconds being approx 180s.
		assert.InDelta(t, 180, savedInvalidTs.SkewSeconds, 5, "Skew should be around 180 seconds")

		// Check event with future end time
		savedInvalidEt, err := store.GetChangeEventByID(ctx, futureEndTimeEvent.ID)
		require.NoError(t, err)
		assert.Equal(t, "invalid_time", savedInvalidEt.Status)
		assert.Equal(t, "end_time_in_future", savedInvalidEt.InvalidReason)
		assert.InDelta(t, 180, savedInvalidEt.SkewSeconds, 5, "Skew should be around 180 seconds")
	})

	// 5. Assertions for Pending Analysis
	t.Run("Verify GetEventsPendingAnalysis only fetches ready events", func(t *testing.T) {
		pendingEvents, err := store.GetEventsPendingAnalysis(ctx)
		require.NoError(t, err)

		// Should only fetch the one valid event
		assert.Len(t, pendingEvents, 1)
		if len(pendingEvents) == 1 {
			assert.Equal(t, validEvent.ID, pendingEvents[0].ID)
		}
	})
}

// TestTimestampGuardrail_EdgeCases tests more specific scenarios and edge cases.
func TestTimestampGuardrail_EdgeCases(t *testing.T) {
	// Setup
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err, "Failed to set up test database")
	defer shared.CleanupTestDB(db, schemaName)

	store := storage.NewPostgresStorage(db, &config.Config{})
	ctx := context.Background()

	// max_future_skew is 2 minutes
	maxSkew := 2 * time.Minute

	t.Run("Event with timestamp exactly at the skew boundary should be ready", func(t *testing.T) {
		boundaryTime := time.Now().Add(maxSkew)
		event := domain.ChangeEvent{
			ID:        "edge-case-boundary-ok",
			Source:    "test-source",
			Timestamp: boundaryTime,
			Summary:   "An event exactly at the future skew boundary.",
		}

		err := store.SaveChangeEvent(ctx, event)
		require.NoError(t, err)

		savedEvent, err := store.GetChangeEventByID(ctx, event.ID)
		require.NoError(t, err)
		assert.Equal(t, "ready", savedEvent.Status)
	})

	t.Run("Event with EndTime exactly at the skew boundary should be ready", func(t *testing.T) {
		boundaryTime := time.Now().Add(maxSkew)
		event := domain.ChangeEvent{
			ID:        "edge-case-endtime-boundary-ok",
			Source:    "test-source",
			Timestamp: time.Now(),
			EndTime:   &boundaryTime,
			Summary:   "An event with EndTime exactly at the future skew boundary.",
		}

		err := store.SaveChangeEvent(ctx, event)
		require.NoError(t, err)

		savedEvent, err := store.GetChangeEventByID(ctx, event.ID)
		require.NoError(t, err)
		assert.Equal(t, "ready", savedEvent.Status)
	})

	t.Run("Event with timestamp just over the skew boundary should be invalid", func(t *testing.T) {
		invalidTime := time.Now().Add(maxSkew).Add(time.Nanosecond)
		event := domain.ChangeEvent{
			ID:        "edge-case-boundary-invalid",
			Source:    "test-source",
			Timestamp: invalidTime,
			Summary:   "An event just over the future skew boundary.",
		}

		err := store.SaveChangeEvent(ctx, event)
		require.NoError(t, err)

		savedEvent, err := store.GetChangeEventByID(ctx, event.ID)
		require.NoError(t, err)
		assert.Equal(t, "invalid_time", savedEvent.Status)
		assert.Equal(t, "timestamp_in_future", savedEvent.InvalidReason)
		assert.InDelta(t, 120, savedEvent.SkewSeconds, 1)
	})

	t.Run("Event with no EndTime should be ready", func(t *testing.T) {
		event := domain.ChangeEvent{
			ID:        "edge-case-no-endtime",
			Source:    "test-source",
			Timestamp: time.Now().Add(-1 * time.Minute),
			EndTime:   nil,
			Summary:   "An event with no end time.",
		}

		err := store.SaveChangeEvent(ctx, event)
		require.NoError(t, err)

		savedEvent, err := store.GetChangeEventByID(ctx, event.ID)
		require.NoError(t, err)
		assert.Equal(t, "ready", savedEvent.Status)
	})

	t.Run("Event updated from valid to invalid", func(t *testing.T) {
		eventID := "update-valid-to-invalid"
		validEvent := domain.ChangeEvent{
			ID:        eventID,
			Source:    "test-source",
			Timestamp: time.Now().Add(-5 * time.Minute),
			Summary:   "Initial valid event.",
		}
		err := store.SaveChangeEvent(ctx, validEvent)
		require.NoError(t, err)

		// Verify it's initially ready
		saved, err := store.GetChangeEventByID(ctx, eventID)
		require.NoError(t, err)
		require.Equal(t, "ready", saved.Status)

		// Now update it with an invalid timestamp
		invalidEvent := validEvent
		invalidEvent.Timestamp = time.Now().Add(3 * time.Minute)
		err = store.SaveChangeEvent(ctx, invalidEvent)
		require.NoError(t, err)

		// Verify it's now invalid
		updated, err := store.GetChangeEventByID(ctx, eventID)
		require.NoError(t, err)
		assert.Equal(t, "invalid_time", updated.Status)
		assert.Equal(t, "timestamp_in_future", updated.InvalidReason)
	})

	t.Run("Event updated from invalid to valid", func(t *testing.T) {
		eventID := "update-invalid-to-valid"
		invalidEvent := domain.ChangeEvent{
			ID:        eventID,
			Source:    "test-source",
			Timestamp: time.Now().Add(3 * time.Minute),
			Summary:   "Initial invalid event.",
		}
		err := store.SaveChangeEvent(ctx, invalidEvent)
		require.NoError(t, err)

		// Verify it's initially invalid
		saved, err := store.GetChangeEventByID(ctx, eventID)
		require.NoError(t, err)
		require.Equal(t, "invalid_time", saved.Status)

		// Now update it with a valid timestamp
		validEvent := invalidEvent
		validEvent.Timestamp = time.Now().Add(-5 * time.Minute)
		err = store.SaveChangeEvent(ctx, validEvent)
		require.NoError(t, err)

		// Verify it's now ready
		updated, err := store.GetChangeEventByID(ctx, eventID)
		require.NoError(t, err)
		assert.Equal(t, "ready", updated.Status)
		assert.Empty(t, updated.InvalidReason)
		assert.Zero(t, updated.SkewSeconds)
	})
}