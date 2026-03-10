package orphandetection

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

func TestOrphanDetection_NoLinkingMetadata(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := &config.Config{}
	cfg.Analysis.IntentExecutionCorrelationDur = 2 * time.Hour

	storage := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	// Create an execution event without any linking metadata
	execEvent := shared.SampleChangeEvent()
	delete(execEvent.Metadata, "git_commit_sha")
	delete(execEvent.Metadata, "image_tag")
	execEvent.Timestamp = time.Now().Add(-1 * time.Hour)

	err = storage.SaveChangeEvent(context.Background(), execEvent)
	require.NoError(t, err)
	
	engine := correlator.NewEngine(storage, mockMetrics, cfg)

	// ACT
	analysis, err := engine.AnalyzeImpact(context.Background(), execEvent)
	require.NoError(t, err)

	// ASSERT
	shared.AssertOrphaned(t, analysis, true)
}