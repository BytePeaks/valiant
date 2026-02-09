package windowclosure

import (
	"context"
	"testing"
	"time"
	"valiant/internal/correlator"
	"valiant/internal/storage"
	"valiant/tests/common"
	"valiant/tests/time-windows/shared"
	"github.com/stretchr/testify/require"
)

func TestImpactWindow_NotClosedError(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	storage := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	// Event happened very recently, so its impact window is still open
	eventTimestamp := time.Now().Add(-10 * time.Minute) // 10 minutes ago
	eventEndTime := eventTimestamp.Add(5 * time.Minute) // Rollout completed 5 minutes ago
	
	event := shared.SampleChangeEvent()
	event.Timestamp = eventTimestamp
	event.EndTime = &eventEndTime

	err = storage.SaveChangeEvent(context.Background(), event)
	require.NoError(t, err)

	engine := correlator.NewEngine(storage, mockMetrics, cfg)

	// ACT
	_, err = engine.AnalyzeImpact(context.Background(), event)

	// ASSERT
	require.ErrorIs(t, err, correlator.ErrImpactWindowNotClosed, "Expected ErrImpactWindowNotClosed for recent event")
	require.Len(t, mockMetrics.Calls, 0, "Expected no calls to GetAverageMetrics when window is not closed")
}
