package scoring

import (
	"context"
	"testing"
	"time"
	"valiant/internal/config"
	"valiant/internal/correlator"
	"valiant/internal/domain"
	"valiant/tests/correlator-engine/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImpactScoring_LevelClassification(t *testing.T) {
	runTest := func(t *testing.T, expectedScore float64, expectedLevel string, metricsFunc func() (domain.MetricValues, domain.MetricValues)) {
		db, schemaName, err := shared.SetupTestDB()
		require.NoError(t, err)
		defer shared.CleanupTestDB(db, schemaName)

		cfg := &config.Config{}
		cfg.Analysis.BaselineDur = 30 * time.Minute
		cfg.Analysis.ImpactDur = 30 * time.Minute
		cfg.Analysis.IntentExecutionCorrelationDur = 1 * time.Hour

		storage := shared.NewTestPostgresStorage(db, cfg)
		mockMetrics := shared.NewMockMetricsProvider()
		baseline, impact := metricsFunc()

		// Set event timestamp far enough in the past so the impact window is already closed.
		// With EndTime at -90m and ImpactDur of 30m, the impact window ends at -90m + 5m + 30m = -55m (in the past). OK.
		eventTimestamp := time.Now().Add(-2 * time.Hour)
		endTime := time.Now().Add(-90 * time.Minute)

		event := domain.ChangeEvent{
			ID:               "test-event-1",
			Source:           "kubernetes",
			TriggerType:      "GitOps",
			ExecutionID:      "deploy-12345",
			ChangeType:       "deployment_rollout",
			Timestamp:        eventTimestamp,
			EndTime:          &endTime,
			AffectedServices: []string{"api-service"},
			Metadata: map[string]string{
				"git_commit_sha": "abc123def456",
				"image_tag":      "v1.2.3",
				"namespace":      "production",
			},
			Summary: "Deployed api-service v1.2.3 to production",
		}

		// Compute time windows from the same timestamp the engine will use
		baselineStartTime := event.Timestamp.Add(-cfg.Analysis.BaselineDur)
		baselineEndTime := event.Timestamp.Add(-5 * time.Minute)

		impactPivot := *event.EndTime
		impactStartTime := impactPivot.Add(5 * time.Minute)
		impactEndTime := impactStartTime.Add(cfg.Analysis.ImpactDur)

		mockMetrics.SetMetrics(
			event.AffectedServices[0],
			baselineStartTime,
			baselineEndTime,
			baseline,
		)
		mockMetrics.SetMetrics(
			event.AffectedServices[0],
			impactStartTime,
			impactEndTime,
			impact,
		)

		err = storage.SaveChangeEvent(context.Background(), event)
		require.NoError(t, err)

		engine := correlator.NewEngine(storage, mockMetrics, cfg)

		analysis, err := engine.AnalyzeImpact(context.Background(), event)
		require.NoError(t, err)

		shared.AssertImpactScore(t, analysis, expectedScore, 0.1)
		shared.AssertImpactLevel(t, analysis, expectedLevel)
		assert.False(t, analysis.IsOrphaned, "Expected event not to be orphaned")
		assert.Equal(t, event.ID, analysis.ChangeEvent.ID, "Expected analyzed event ID to match original event ID")
	}

	t.Run("NoImpact", func(t *testing.T) {
		runTest(t, 0.0, "NONE", shared.NoImpactMetrics)
	})
	t.Run("LowImpact", func(t *testing.T) {
		runTest(t, 0.2, "LOW", shared.LowImpactMetrics)
	})
	t.Run("MediumImpact", func(t *testing.T) {
		runTest(t, 0.5, "MEDIUM", shared.MediumImpactMetrics)
	})
	t.Run("HighImpact", func(t *testing.T) {
		runTest(t, 0.8, "HIGH", shared.HighImpactMetrics)
	})
}
