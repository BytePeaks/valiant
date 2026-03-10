package determinism

import (
	"context"
	"fmt"
	"testing"
	"time"
	"valiant/internal/config"
	"valiant/internal/correlator"
	"valiant/internal/storage"
	"valiant/tests/correlator-engine/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeterminism_SameInputSameOutput(t *testing.T) {
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := &config.Config{}
	cfg.Analysis.BaselineDur = 30 * time.Minute
	cfg.Analysis.ImpactDur = 30 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := shared.NewMockMetricsProvider()

	eventTimestamp := time.Now().Add(-2 * time.Hour)
	endTime := time.Now().Add(-90 * time.Minute)
	baseline, impact := shared.LowImpactMetrics()

	// Register mock metrics for the expected time windows
	baselineStart := eventTimestamp.Add(-cfg.Analysis.BaselineDur)
	baselineEnd := eventTimestamp.Add(-5 * time.Minute)
	impactStart := endTime.Add(5 * time.Minute)
	impactEnd := impactStart.Add(cfg.Analysis.ImpactDur)

	// Run with 5 different event IDs, same timestamps and metrics
	var scores []float64
	var levels []string
	var confidences []float64

	for i := 0; i < 5; i++ {
		eventID := fmt.Sprintf("test-determ-%d", i)

		event := shared.SampleChangeEvent()
		event.ID = eventID
		event.Timestamp = eventTimestamp
		event.EndTime = &endTime

		mockMetrics.SetMetrics(event.AffectedServices[0], baselineStart, baselineEnd, baseline)
		mockMetrics.SetMetrics(event.AffectedServices[0], impactStart, impactEnd, impact)

		err = store.SaveChangeEvent(context.Background(), event)
		require.NoError(t, err)

		engine := correlator.NewEngine(store, mockMetrics, cfg)
		analysis, err := engine.AnalyzeImpact(context.Background(), event)
		require.NoError(t, err)

		scores = append(scores, analysis.ImpactScore)
		levels = append(levels, analysis.ImpactLevel)
		confidences = append(confidences, analysis.ConfidenceScore)
	}

	// All scores should be identical
	for i := 1; i < len(scores); i++ {
		assert.Equal(t, scores[0], scores[i], "ImpactScore mismatch on run %d", i)
		assert.Equal(t, levels[0], levels[i], "ImpactLevel mismatch on run %d", i)
		assert.Equal(t, confidences[0], confidences[i], "ConfidenceScore mismatch on run %d", i)
	}
}

func TestDeterminism_ScoreInvariantToWallClock(t *testing.T) {
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := &config.Config{}
	cfg.Analysis.BaselineDur = 30 * time.Minute
	cfg.Analysis.ImpactDur = 30 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := shared.NewMockMetricsProvider()

	baseline, impact := shared.MediumImpactMetrics()

	// Two events at different absolute times
	timestamps := []time.Time{
		time.Now().Add(-3 * time.Hour),
		time.Now().Add(-5 * time.Hour),
	}

	var scores []float64
	for i, ts := range timestamps {
		eventID := fmt.Sprintf("test-wallclock-%d", i)
		endTs := ts.Add(10 * time.Minute)

		event := shared.SampleChangeEvent()
		event.ID = eventID
		event.Timestamp = ts
		event.EndTime = &endTs

		baselineStart := ts.Add(-cfg.Analysis.BaselineDur)
		baselineEnd := ts.Add(-5 * time.Minute)
		impactStart := endTs.Add(5 * time.Minute)
		impactEnd := impactStart.Add(cfg.Analysis.ImpactDur)

		mockMetrics.SetMetrics(event.AffectedServices[0], baselineStart, baselineEnd, baseline)
		mockMetrics.SetMetrics(event.AffectedServices[0], impactStart, impactEnd, impact)

		err = store.SaveChangeEvent(context.Background(), event)
		require.NoError(t, err)

		engine := correlator.NewEngine(store, mockMetrics, cfg)
		analysis, err := engine.AnalyzeImpact(context.Background(), event)
		require.NoError(t, err)

		scores = append(scores, analysis.ImpactScore)
	}

	assert.Equal(t, scores[0], scores[1], "Score should be identical regardless of absolute wall-clock time")
}
