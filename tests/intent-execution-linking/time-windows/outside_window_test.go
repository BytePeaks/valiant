package timewindows

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

func TestLinking_OutsideCorrelationWindow(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	// Use a 2-hour correlation window for this test
	cfg := &config.Config{}
	cfg.Analysis.IntentExecutionCorrelationDur = 2 * time.Hour
	cfg.Analysis.BaselineDur = 30 * time.Minute
	cfg.Analysis.ImpactDur = 30 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	// CI event from 3 hours ago (outside the 2h window)
	ciEvent := common.SampleCIEvent()
	ciEvent.Timestamp = time.Now().Add(-3 * time.Hour).Add(-1 * time.Minute)
	ciEvent.Metadata["git_commit_sha"] = "abc123def456"

	// Execution event from 30 minutes ago
	execEvent := common.SampleChangeEvent()
	execEvent.Timestamp = time.Now().Add(-60 * time.Minute)
	execEndTime := execEvent.Timestamp.Add(5 * time.Minute)
	execEvent.EndTime = &execEndTime
	execEvent.Metadata["git_commit_sha"] = "abc123def456"

	err = store.SaveChangeEvent(context.Background(), ciEvent)
	require.NoError(t, err)
	err = store.SaveChangeEvent(context.Background(), execEvent)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)

	// ACT
	analysis, err := engine.AnalyzeImpact(context.Background(), execEvent)
	require.NoError(t, err)

	// ASSERT
	// The execution event should be orphaned because the CI event is too old.
	shared.AssertOrphaned(t, analysis, true)
	require.Nil(t, analysis.LinkedCIEvent, "LinkedCIEvent should be nil for an orphaned event")
}
