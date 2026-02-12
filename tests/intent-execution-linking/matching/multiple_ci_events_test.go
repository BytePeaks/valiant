package matching

import (
	"context"
	"testing"
	"time"
	"valiant/internal/config"
	"valiant/internal/correlator"
	"valiant/internal/storage"
	"valiant/tests/common"
	"valiant/tests/intent-execution-linking/shared"
	"github.com/stretchr/testify/require"
)

func TestSuccessfulLinking_MultipleCIEvents(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := &config.Config{}
	cfg.Analysis.IntentExecutionCorrelationDur = 4 * time.Hour // Wider window to include both CI events
	cfg.Analysis.BaselineDur = 30 * time.Minute
	cfg.Analysis.ImpactDur = 30 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	const matchingSha = "def456abc123"

	// Old CI event
	oldCiEvent := common.SampleCIEvent()
	oldCiEvent.ID = "ci-event-old"
	oldCiEvent.Timestamp = time.Now().Add(-3 * time.Hour)
	oldCiEvent.Metadata["git_commit_sha"] = matchingSha

	// New CI event, should be preferred
	newCiEvent := common.SampleCIEvent()
	newCiEvent.ID = "ci-event-new"
	newCiEvent.Timestamp = time.Now().Add(-2 * time.Hour)
	newCiEvent.Metadata["git_commit_sha"] = matchingSha

	// Execution event
	execEvent := common.SampleChangeEvent()
	execEvent.Timestamp = time.Now().Add(-1 * time.Hour)
	execEndTime := execEvent.Timestamp.Add(5 * time.Minute)
	execEvent.EndTime = &execEndTime
	execEvent.Metadata["git_commit_sha"] = matchingSha

	err = store.SaveChangeEvent(context.Background(), oldCiEvent)
	require.NoError(t, err)
	err = store.SaveChangeEvent(context.Background(), newCiEvent)
	require.NoError(t, err)
	err = store.SaveChangeEvent(context.Background(), execEvent)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)

	// ACT
	analysis, err := engine.AnalyzeImpact(context.Background(), execEvent)
	require.NoError(t, err)

	// ASSERT
	// Should not be orphaned and should link to the newest CI event.
	shared.AssertOrphaned(t, analysis, false)
	require.NotNil(t, analysis.LinkedCIEvent, "Execution event should be linked")
	require.Equal(t, newCiEvent.ID, analysis.LinkedCIEvent.ID, "Should link to the newest CI event")
}
