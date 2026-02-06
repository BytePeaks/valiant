package impact

import (
	"context"
	"testing"
	"time"
	"valiant/internal/config"
	"valiant/internal/correlator"
	"valiant/internal/storage"
	"valiant/tests/time-windows/shared"
	"github.com/stretchr/testify/require"
)

func TestConfigurableImpactDuration(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	storage := storage.NewPostgresStorage(db)
	mockMetrics := &shared.MockMetricsProvider{}

	eventTimestamp := time.Now().Add(-10 * time.Hour) // A sufficiently old event
	eventEndTime := eventTimestamp.Add(1 * time.Minute)
	
	event := shared.SampleChangeEvent()
	event.Timestamp = eventTimestamp
	event.EndTime = &eventEndTime

	err = storage.SaveChangeEvent(context.Background(), event)
	require.NoError(t, err)

	// Use a custom duration for the impact window
	customImpactDur := 1 * time.Hour
	
	cfg := &config.Config{}
	cfg.Analysis.BaselineDur = 30 * time.Minute
	cfg.Analysis.ImpactDur = customImpactDur

	engine := correlator.NewEngine(storage, mockMetrics, cfg)

	// ACT
	_, err = engine.AnalyzeImpact(context.Background(), event)
	require.NoError(t, err)

	// ASSERT
	require.Len(t, mockMetrics.Calls, 2, "Expected 2 calls to GetAverageMetrics")

	// Assert that the impact call used the custom duration
	impactCall := mockMetrics.Calls[1]
	impactPivot := *event.EndTime
	expectedImpactStart := impactPivot.Add(5 * time.Minute)
	expectedImpactEnd := expectedImpactStart.Add(customImpactDur)

	shared.AssertTimeEqual(t, impactCall.Start, expectedImpactStart, time.Second, "Impact Start Time Mismatch with custom duration")
	shared.AssertTimeEqual(t, impactCall.End, expectedImpactEnd, time.Second, "Impact End Time Mismatch with custom duration")
}
