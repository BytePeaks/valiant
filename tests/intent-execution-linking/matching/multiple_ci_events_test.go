package matching

import (
	"context"
	"testing"
	"time"
	"valiant/internal/config"
	"valiant/internal/correlator"
	"valiant/internal/storage"
	"valiant/tests/intent-execution-linking/shared"
	"github.com/stretchr/testify/require"
)

func TestSuccessfulLinking_MultipleCIEvents(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	storage := storage.NewPostgresStorage(db)
	mockMetrics := &shared.MockMetricsProvider{}

	// Create two CI events with the same SHA, one older than the other
	sha := "abcdef123456"
	oldCiEvent := shared.SampleCIEvent()
	oldCiEvent.ID = "test-ci-old"
	oldCiEvent.Metadata["git_commit_sha"] = sha
	oldCiEvent.Timestamp = time.Now().Add(-3 * time.Hour)

	newCiEvent := shared.SampleCIEvent()
	newCiEvent.ID = "test-ci-new"
	newCiEvent.Metadata["git_commit_sha"] = sha
	newCiEvent.Timestamp = time.Now().Add(-2 * time.Hour)

	execEvent := shared.SampleChangeEvent()
	execEvent.Metadata["git_commit_sha"] = sha
	execEvent.Timestamp = time.Now().Add(-1 * time.Hour)

	// Save all events
	err = storage.SaveChangeEvent(context.Background(), oldCiEvent)
	require.NoError(t, err)
	err = storage.SaveChangeEvent(context.Background(), newCiEvent)
	require.NoError(t, err)
	err = storage.SaveChangeEvent(context.Background(), execEvent)
	require.NoError(t, err)
	
	cfg := &config.Config{}
	cfg.Analysis.IntentExecutionCorrelationDur = 4 * time.Hour // Wider window to include both CI events

	engine := correlator.NewEngine(storage, mockMetrics, cfg)

	// ACT
	analysis, err := engine.AnalyzeImpact(context.Background(), execEvent)
	require.NoError(t, err)

	// ASSERT
	// Should not be orphaned as at least one matching CI event exists within the window.
	// The engine should ideally link to the newest one (newCiEvent).
	shared.AssertOrphaned(t, analysis, false)
}
