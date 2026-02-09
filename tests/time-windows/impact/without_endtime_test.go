package impact

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

func TestImpactWindow_WithoutEndTime(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	storage := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	eventTimestamp := time.Now().Add(-5 * time.Hour) // Event happened 5 hours ago
	
	event := shared.SampleChangeEvent()
	event.Timestamp = eventTimestamp
	event.EndTime = nil // Crucial: EndTime is nil

	err = storage.SaveChangeEvent(context.Background(), event)
	require.NoError(t, err)

	engine := correlator.NewEngine(storage, mockMetrics, cfg)

	// ACT
	_, err = engine.AnalyzeImpact(context.Background(), event)
	require.NoError(t, err) 

	// ASSERT
	require.Len(t, mockMetrics.Calls, 2, "Expected 2 calls to GetAverageMetrics (for baseline and impact)")

	// Assert Impact Window uses event.Timestamp as pivot
	impactCall := mockMetrics.Calls[1]
	impactPivot := eventTimestamp // Should use Timestamp when EndTime is nil
	expectedImpactStart := impactPivot.Add(5 * time.Minute)
	expectedImpactEnd := expectedImpactStart.Add(cfg.Analysis.ImpactDur)
	shared.AssertTimeEqual(t, impactCall.Start, expectedImpactStart, time.Second, "Impact Start Time Mismatch")
	shared.AssertTimeEqual(t, impactCall.End, expectedImpactEnd, time.Second, "Impact End Time Mismatch")
}
