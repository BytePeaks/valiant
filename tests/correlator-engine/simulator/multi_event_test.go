package simulator

import (
	"context"
	"fmt"
	"testing"
	"time"
	"valiant/internal/config"
	"valiant/internal/correlator"
	"valiant/internal/domain"
	"valiant/internal/storage"
	"valiant/tests/correlator-engine/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultipleChanges_RankedByLikelihood(t *testing.T) {
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := &config.Config{}
	cfg.Analysis.BaselineDur = 30 * time.Minute
	cfg.Analysis.ImpactDur = 30 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := shared.NewMockMetricsProvider()

	baseTime := time.Now().Add(-3 * time.Hour)

	// Create 3 events for the same service at different times
	events := []domain.ChangeEvent{
		{
			ID:               "sim-deploy-1",
			TriggerType:      "GitOps",
			ChangeType:       "deployment_rollout",
			Timestamp:        baseTime,
			AffectedServices: []string{"api-service"},
			Metadata:         map[string]string{"git_commit_sha": "aaa"},
			Summary:          "Deploy 1",
		},
		{
			ID:               "sim-configmap-1",
			TriggerType:      "GitOps",
			ChangeType:       "configmap_update",
			Timestamp:        baseTime.Add(5 * time.Minute),
			AffectedServices: []string{"api-service"},
			Metadata:         map[string]string{"git_commit_sha": "bbb"},
			Summary:          "ConfigMap update",
		},
		{
			ID:               "sim-deploy-2",
			TriggerType:      "GitOps",
			ChangeType:       "deployment_rollout",
			Timestamp:        baseTime.Add(10 * time.Minute),
			AffectedServices: []string{"api-service"},
			Metadata:         map[string]string{"git_commit_sha": "ccc"},
			Summary:          "Deploy 2",
		},
	}

	_, highImpact := shared.HighImpactMetrics()
	lowBaseline, _ := shared.NoImpactMetrics()

	for _, event := range events {
		err = store.SaveChangeEvent(context.Background(), event)
		require.NoError(t, err)

		// Set up metrics so each event gets analyzed
		bStart := event.Timestamp.Add(-cfg.Analysis.BaselineDur)
		bEnd := event.Timestamp.Add(-5 * time.Minute)
		iStart := event.Timestamp.Add(5 * time.Minute)
		iEnd := iStart.Add(cfg.Analysis.ImpactDur)

		mockMetrics.SetMetrics("api-service", bStart, bEnd, lowBaseline)
		mockMetrics.SetMetrics("api-service", iStart, iEnd, highImpact)
	}

	engine := correlator.NewEngine(store, mockMetrics, cfg)

	// Rank all changes within the time window
	from := baseTime.Add(-10 * time.Minute)
	to := baseTime.Add(20 * time.Minute)
	ranked, err := engine.RankChanges(context.Background(), "api-service", from, to)
	require.NoError(t, err)

	assert.Len(t, ranked, 3, "Expected 3 ranked changes")

	// Verify ranks are sequential
	for i, r := range ranked {
		assert.Equal(t, i+1, r.Rank, "Expected rank %d", i+1)
	}

	// deployment_rollout should rank higher than configmap_update (weight 1.0 vs 0.5)
	// Both deployments have weight 1.0, so the one closer to midpoint ranks higher
	assert.True(t, ranked[0].LikelihoodScore >= ranked[1].LikelihoodScore,
		"First ranked should have highest likelihood")
	assert.True(t, ranked[1].LikelihoodScore >= ranked[2].LikelihoodScore,
		"Second ranked should have higher likelihood than third")
}

func TestCascadeScenario_CrossServiceImpact(t *testing.T) {
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := &config.Config{}
	cfg.Analysis.BaselineDur = 30 * time.Minute
	cfg.Analysis.ImpactDur = 30 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := shared.NewMockMetricsProvider()

	baseTime := time.Now().Add(-3 * time.Hour)

	// Service A change that causes service B degradation
	event := domain.ChangeEvent{
		ID:               "cascade-db-change",
		TriggerType:      "GitOps",
		ChangeType:       "deployment_rollout",
		Timestamp:        baseTime,
		AffectedServices: []string{"database"},
		Metadata:         map[string]string{"git_commit_sha": "cascade1"},
		Summary:          "Database migration deploy",
	}

	err = store.SaveChangeEvent(context.Background(), event)
	require.NoError(t, err)

	baseline := shared.SampleMetricValues(0.01, 100, 100, 50, 60)
	impact := shared.SampleMetricValues(0.01, 100, 100, 50, 60)

	bStart := event.Timestamp.Add(-cfg.Analysis.BaselineDur)
	bEnd := event.Timestamp.Add(-5 * time.Minute)
	iStart := event.Timestamp.Add(5 * time.Minute)
	iEnd := iStart.Add(cfg.Analysis.ImpactDur)

	mockMetrics.SetMetrics("database", bStart, bEnd, baseline)
	mockMetrics.SetMetrics("database", iStart, iEnd, impact)

	engine := correlator.NewEngine(store, mockMetrics, cfg)

	// When querying for "api-service" (not directly affected), event should still appear
	// with indirect service scope (0.5)
	from := baseTime.Add(-10 * time.Minute)
	to := baseTime.Add(20 * time.Minute)
	ranked, err := engine.RankChanges(context.Background(), "api-service", from, to)
	require.NoError(t, err)

	require.Len(t, ranked, 1, "Database change should appear in api-service ranking")
	assert.Equal(t, 0.5, ranked[0].ServiceScope,
		"Database change should have indirect service scope for api-service")
}

func TestHighVolume_ManyEvents(t *testing.T) {
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := &config.Config{}
	cfg.Analysis.BaselineDur = 30 * time.Minute
	cfg.Analysis.ImpactDur = 30 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := shared.NewMockMetricsProvider()

	baseTime := time.Now().Add(-5 * time.Hour)
	baseline := shared.SampleMetricValues(0.01, 100, 100, 50, 60)
	impact := shared.SampleMetricValues(0.02, 110, 95, 52, 62)

	// Create 20 events spread over 2 hours
	for i := 0; i < 20; i++ {
		ts := baseTime.Add(time.Duration(i) * 6 * time.Minute)
		event := domain.ChangeEvent{
			ID:               fmt.Sprintf("highvol-%d", i),
			TriggerType:      "GitOps",
			ChangeType:       "deployment_rollout",
			Timestamp:        ts,
			AffectedServices: []string{"api-service"},
			Metadata:         map[string]string{"git_commit_sha": fmt.Sprintf("sha%d", i)},
			Summary:          fmt.Sprintf("Deploy %d", i),
		}

		err = store.SaveChangeEvent(context.Background(), event)
		require.NoError(t, err)

		bStart := ts.Add(-cfg.Analysis.BaselineDur)
		bEnd := ts.Add(-5 * time.Minute)
		iStart := ts.Add(5 * time.Minute)
		iEnd := iStart.Add(cfg.Analysis.ImpactDur)

		mockMetrics.SetMetrics("api-service", bStart, bEnd, baseline)
		mockMetrics.SetMetrics("api-service", iStart, iEnd, impact)
	}

	engine := correlator.NewEngine(store, mockMetrics, cfg)

	from := baseTime.Add(-10 * time.Minute)
	to := baseTime.Add(130 * time.Minute)
	ranked, err := engine.RankChanges(context.Background(), "api-service", from, to)
	require.NoError(t, err)

	assert.Equal(t, 20, len(ranked), "All 20 events should be ranked")

	// Verify ranks are sequential and sorted by likelihood descending
	for i := 0; i < len(ranked)-1; i++ {
		assert.Equal(t, i+1, ranked[i].Rank)
		assert.GreaterOrEqual(t, ranked[i].LikelihoodScore, ranked[i+1].LikelihoodScore,
			"Events should be sorted by likelihood descending")
	}
}
