package timewindows

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

func TestLinking_OutsideCorrelationWindow(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	storage := storage.NewPostgresStorage(db)
	mockMetrics := &shared.MockMetricsProvider{}

	// Create a CI event and a GitOps event with the same commit SHA
	sha := "abcdef123456"
	ciEvent := shared.SampleCIEvent()
	ciEvent.Metadata["git_commit_sha"] = sha
	ciEvent.Timestamp = time.Now().Add(-4 * time.Hour) // 4 hours ago

	execEvent := shared.SampleChangeEvent()
	execEvent.Metadata["git_commit_sha"] = sha
	execEvent.Timestamp = time.Now().Add(-1 * time.Hour) // 1 hour ago, outside the 2h window

	err = storage.SaveChangeEvent(context.Background(), ciEvent)
	require.NoError(t, err)
	err = storage.SaveChangeEvent(context.Background(), execEvent)
	require.NoError(t, err)

	// Correlator engine with a 2-hour correlation window
	cfg := &config.Config{}
	cfg.Analysis.IntentExecutionCorrelationDur = 2 * time.Hour

	engine := correlator.NewEngine(storage, mockMetrics, cfg)

	// ACT
	analysis, err := engine.AnalyzeImpact(context.Background(), execEvent)
	require.NoError(t, err)

	// ASSERT
	// The execution event should be orphaned because the CI event is too old.
	shared.AssertOrphaned(t, analysis, true)
}
