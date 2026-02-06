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

func TestOrphanDetection_NoMatchingCIEvent(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	storage := storage.NewPostgresStorage(db)
	mockMetrics := &shared.MockMetricsProvider{}

	// Create only an execution event, with no corresponding CI event
	execEvent := shared.SampleChangeEvent()
	execEvent.Timestamp = time.Now().Add(-1 * time.Hour)

	err = storage.SaveChangeEvent(context.Background(), execEvent)
	require.NoError(t, err)
	
	cfg := &config.Config{}
	cfg.Analysis.IntentExecutionCorrelationDur = 2 * time.Hour

	engine := correlator.NewEngine(storage, mockMetrics, cfg)

	// ACT
	analysis, err := engine.AnalyzeImpact(context.Background(), execEvent)
	require.NoError(t, err)

	// ASSERT
	shared.AssertOrphaned(t, analysis, true)
}
