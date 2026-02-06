package baseline

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

func TestStandardBaselineWindow(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	storage := storage.NewPostgresStorage(db)
	mockMetrics := &shared.MockMetricsProvider{}

	eventTimestamp := time.Now().Add(-5 * time.Hour) // Event happened 5 hours ago
	eventEndTime := eventTimestamp.Add(10 * time.Minute) // Rollout took 10 minutes
	
	event := shared.SampleChangeEvent()
	event.Timestamp = eventTimestamp
	event.EndTime = &eventEndTime

	err = storage.SaveChangeEvent(context.Background(), event)
	require.NoError(t, err)

	cfg := &config.Config{}
	cfg.Analysis.BaselineDur = 30 * time.Minute
	cfg.Analysis.ImpactDur = 30 * time.Minute

	engine := correlator.NewEngine(storage, mockMetrics, cfg)

	// ACT
	_, err = engine.AnalyzeImpact(context.Background(), event)
	require.NoError(t, err) // Should not return ErrImpactWindowNotClosed for this old event

	// ASSERT
	require.Len(t, mockMetrics.Calls, 2, "Expected 2 calls to GetAverageMetrics (for baseline and impact)")

	// Assert Baseline Window
	baselineCall := mockMetrics.Calls[0]
	expectedBaselineStart := eventTimestamp.Add(-cfg.Analysis.BaselineDur)
	expectedBaselineEnd := eventTimestamp.Add(-5 * time.Minute)
	shared.AssertTimeEqual(t, baselineCall.Start, expectedBaselineStart, time.Second, "Baseline Start Time Mismatch")
	shared.AssertTimeEqual(t, baselineCall.End, expectedBaselineEnd, time.Second, "Baseline End Time Mismatch")

	// Assert Impact Window
	impactCall := mockMetrics.Calls[1]
	impactPivot := *event.EndTime // Use EndTime for impact pivot
	expectedImpactStart := impactPivot.Add(5 * time.Minute)
	expectedImpactEnd := expectedImpactStart.Add(cfg.Analysis.ImpactDur)
	shared.AssertTimeEqual(t, impactCall.Start, expectedImpactStart, time.Second, "Impact Start Time Mismatch")
	shared.AssertTimeEqual(t, impactCall.End, expectedImpactEnd, time.Second, "Impact End Time Mismatch")
}
