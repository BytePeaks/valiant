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

func TestSuccessfulLinking_ShaMatch(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := &config.Config{}
	cfg.Analysis.IntentExecutionCorrelationDur = 2 * time.Hour
	cfg.Analysis.BaselineDur = 30 * time.Minute
	cfg.Analysis.ImpactDur = 30 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	const matchingSha = "abc123def456"

	// CI event with the commit SHA
	ciEvent := common.SampleCIEvent()
	ciEvent.Timestamp = time.Now().Add(-1 * time.Hour)
	ciEvent.Metadata["git_commit_sha"] = matchingSha

	// Execution event with the same commit SHA (must be old enough for impact window to close)
	execEvent := common.SampleChangeEvent()
	execEvent.Timestamp = time.Now().Add(-50 * time.Minute)
	execEndTime := execEvent.Timestamp.Add(5 * time.Minute)
	execEvent.EndTime = &execEndTime
	execEvent.Metadata["git_commit_sha"] = matchingSha

	err = store.SaveChangeEvent(context.Background(), ciEvent)
	require.NoError(t, err)
	err = store.SaveChangeEvent(context.Background(), execEvent)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)

	// ACT
	analysis, err := engine.AnalyzeImpact(context.Background(), execEvent)
	require.NoError(t, err)

	// ASSERT
	shared.AssertOrphaned(t, analysis, false)
	require.NotNil(t, analysis.LinkedCIEvent, "Execution event should be linked to a CI event")
	require.Equal(t, ciEvent.ID, analysis.LinkedCIEvent.ID, "Linked CI event ID does not match")
	require.Equal(t, matchingSha, analysis.LinkedCIEvent.Metadata["git_commit_sha"])
}
