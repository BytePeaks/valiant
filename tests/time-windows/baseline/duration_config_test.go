package baseline

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

func TestConfigurableBaselineDuration(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	// Use a custom duration for the baseline window
	customBaselineDur := 1 * time.Hour
	cfg.Analysis.BaselineDur = customBaselineDur
	cfg.Analysis.BaselineWindow = customBaselineDur.String() // Also update the string representation

	storage := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	eventTimestamp := time.Now().Add(-10 * time.Hour) // A sufficiently old event
	event := shared.SampleChangeEvent()
	event.Timestamp = eventTimestamp
	
	// Set EndTime to be far in the past to ensure window is closed
	eventEndTime := eventTimestamp.Add(1 * time.Minute)
	event.EndTime = &eventEndTime


	err = storage.SaveChangeEvent(context.Background(), event)
	require.NoError(t, err)

	engine := correlator.NewEngine(storage, mockMetrics, cfg)

	// ACT
	_, err = engine.AnalyzeImpact(context.Background(), event)
	require.NoError(t, err)

	// ASSERT
	require.Len(t, mockMetrics.Calls, 2, "Expected 2 calls to GetAverageMetrics")

	// Assert that the baseline call used the custom duration
	baselineCall := mockMetrics.Calls[0]
	expectedBaselineStart := eventTimestamp.Add(-customBaselineDur)
	expectedBaselineEnd := eventTimestamp.Add(-5 * time.Minute) // This is hardcoded in the engine

	shared.AssertTimeEqual(t, baselineCall.Start, expectedBaselineStart, time.Second, "Baseline Start Time Mismatch with custom duration")
	shared.AssertTimeEqual(t, baselineCall.End, expectedBaselineEnd, time.Second, "Baseline End Time Mismatch with custom duration")
}
