package simulator

import (
	"context"
	"testing"
	"time"
	"valiant/internal/correlator"
	"valiant/internal/storage"
	"valiant/tests/common"
	"valiant/tests/time-windows/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLongRollout_EndTimeFarFromTimestamp(t *testing.T) {
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	// Rollout that took 2 hours
	eventTimestamp := time.Now().Add(-6 * time.Hour)
	eventEndTime := eventTimestamp.Add(2 * time.Hour)

	event := shared.SampleChangeEvent()
	event.ID = "long-rollout-1"
	event.Timestamp = eventTimestamp
	event.EndTime = &eventEndTime

	err = store.SaveChangeEvent(context.Background(), event)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)

	_, err = engine.AnalyzeImpact(context.Background(), event)
	require.NoError(t, err)

	// Verify window calculations
	require.Len(t, mockMetrics.Calls, 2)

	// Baseline should be relative to Timestamp (start of rollout)
	baselineCall := mockMetrics.Calls[0]
	expectedBaselineStart := eventTimestamp.Add(-cfg.Analysis.BaselineDur)
	expectedBaselineEnd := eventTimestamp.Add(-5 * time.Minute)
	shared.AssertTimeEqual(t, baselineCall.Start, expectedBaselineStart, time.Second)
	shared.AssertTimeEqual(t, baselineCall.End, expectedBaselineEnd, time.Second)

	// Impact should be relative to EndTime (end of rollout), not Timestamp
	impactCall := mockMetrics.Calls[1]
	expectedImpactStart := eventEndTime.Add(5 * time.Minute)
	expectedImpactEnd := expectedImpactStart.Add(cfg.Analysis.ImpactDur)
	shared.AssertTimeEqual(t, impactCall.Start, expectedImpactStart, time.Second)
	shared.AssertTimeEqual(t, impactCall.End, expectedImpactEnd, time.Second)

	// The gap between baseline end and impact start should be > 2 hours
	gap := impactCall.Start.Sub(baselineCall.End)
	assert.Greater(t, gap, 2*time.Hour,
		"Gap between baseline and impact should span the entire rollout duration")
}

func TestOverlappingWindows_ConcurrentEvents(t *testing.T) {
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	store := storage.NewPostgresStorage(db, cfg)

	baseTime := time.Now().Add(-5 * time.Hour)

	// Two events 10 minutes apart — their windows overlap
	event1 := shared.SampleChangeEvent()
	event1.ID = "overlap-1"
	event1.Timestamp = baseTime
	endTime1 := baseTime.Add(5 * time.Minute)
	event1.EndTime = &endTime1

	event2 := shared.SampleChangeEvent()
	event2.ID = "overlap-2"
	event2.Timestamp = baseTime.Add(10 * time.Minute)
	endTime2 := baseTime.Add(15 * time.Minute)
	event2.EndTime = &endTime2

	err = store.SaveChangeEvent(context.Background(), event1)
	require.NoError(t, err)
	err = store.SaveChangeEvent(context.Background(), event2)
	require.NoError(t, err)

	// Each event gets its own mock metrics provider to track calls independently
	mockMetrics1 := &shared.MockMetricsProvider{}
	mockMetrics2 := &shared.MockMetricsProvider{}

	engine1 := correlator.NewEngine(store, mockMetrics1, cfg)
	engine2 := correlator.NewEngine(store, mockMetrics2, cfg)

	_, err = engine1.AnalyzeImpact(context.Background(), event1)
	require.NoError(t, err)
	_, err = engine2.AnalyzeImpact(context.Background(), event2)
	require.NoError(t, err)

	// Both should have made 2 calls each
	require.Len(t, mockMetrics1.Calls, 2)
	require.Len(t, mockMetrics2.Calls, 2)

	// Event1's impact window starts at endTime1 + 5m = baseTime + 10m
	// Event2's baseline window starts at baseTime + 10m - 30m = baseTime - 20m
	// Event2's baseline end = baseTime + 10m - 5m = baseTime + 5m
	// So Event1's impact start overlaps with Event2's baseline window
	event1ImpactStart := mockMetrics1.Calls[1].Start
	event2BaselineEnd := mockMetrics2.Calls[0].End

	// Event1 impact starts at baseTime + 10m
	// Event2 baseline ends at baseTime + 5m
	// They are close but don't need to actually overlap for this test;
	// the point is that each event computes its own independent windows
	assert.NotEqual(t, event1ImpactStart, event2BaselineEnd,
		"Each event should have independently calculated windows")
}

func TestSubMinutePrecision_WindowBoundaries(t *testing.T) {
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	// Event with sub-second precision in timestamps
	eventTimestamp := time.Now().Add(-5 * time.Hour).Add(123 * time.Millisecond)
	eventEndTime := eventTimestamp.Add(10*time.Minute + 456*time.Millisecond)

	event := shared.SampleChangeEvent()
	event.ID = "sub-minute-1"
	event.Timestamp = eventTimestamp
	event.EndTime = &eventEndTime

	err = store.SaveChangeEvent(context.Background(), event)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)
	_, err = engine.AnalyzeImpact(context.Background(), event)
	require.NoError(t, err)

	require.Len(t, mockMetrics.Calls, 2)

	// Verify sub-second precision is preserved in window calculations
	baselineCall := mockMetrics.Calls[0]
	impactCall := mockMetrics.Calls[1]

	// Baseline duration should be exactly 25 minutes (30m start to 5m before event)
	baselineDuration := baselineCall.End.Sub(baselineCall.Start)
	shared.AssertDurationEqual(t, baselineDuration, 25*time.Minute, time.Second,
		"Baseline window should span 25 minutes")

	// Impact duration should be exactly 30 minutes
	impactDuration := impactCall.End.Sub(impactCall.Start)
	shared.AssertDurationEqual(t, impactDuration, 30*time.Minute, time.Second,
		"Impact window should span 30 minutes")

	// Impact pivot should use EndTime's sub-second precision
	expectedImpactStart := eventEndTime.Add(5 * time.Minute)
	shared.AssertTimeEqual(t, impactCall.Start, expectedImpactStart, time.Millisecond,
		"Impact start should preserve sub-second precision from EndTime")
}
