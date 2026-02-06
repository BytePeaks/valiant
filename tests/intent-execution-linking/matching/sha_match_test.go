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

func TestSuccessfulLinking_ShaMatch(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	// Create storage and mock metrics provider
	storage := storage.NewPostgresStorage(db)
	mockMetrics := &shared.MockMetricsProvider{}

	// Create a CI event and a GitOps event with the same commit SHA
	sha := "abcdef123456"
	ciEvent := shared.SampleCIEvent()
	ciEvent.Metadata["git_commit_sha"] = sha
	ciEvent.Timestamp = time.Now().Add(-2 * time.Hour)

	execEvent := shared.SampleChangeEvent()
	execEvent.Metadata["git_commit_sha"] = sha
	execEvent.Timestamp = time.Now().Add(-1 * time.Hour) // Occurs after CI event

	// Save both events to the database
	err = storage.SaveChangeEvent(context.Background(), ciEvent)
	require.NoError(t, err)
	err = storage.SaveChangeEvent(context.Background(), execEvent)
	require.NoError(t, err)
	
	// Create correlator engine
	cfg := &config.Config{}
	cfg.Analysis.IntentExecutionCorrelationDur = 2 * time.Hour

	engine := correlator.NewEngine(storage, mockMetrics, cfg)

	// ACT
	analysis, err := engine.AnalyzeImpact(context.Background(), execEvent)
	require.NoError(t, err)

	// ASSERT
	shared.AssertOrphaned(t, analysis, false)
}
