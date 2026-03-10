package simulator

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

func TestMultipleExecutions_SingleCIEvent(t *testing.T) {
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := &config.Config{}
	cfg.Analysis.IntentExecutionCorrelationDur = 2 * time.Hour

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	sha := "multi-exec-sha-123"
	ciEvent := shared.SampleCIEvent()
	ciEvent.ID = "ci-multi-exec"
	ciEvent.Metadata["git_commit_sha"] = sha
	ciEvent.Timestamp = time.Now().Add(-3 * time.Hour)

	err = store.SaveChangeEvent(context.Background(), ciEvent)
	require.NoError(t, err)

	// Two execution events referencing the same CI event SHA
	for i, id := range []string{"exec-staging", "exec-prod"} {
		execEvent := shared.SampleChangeEvent()
		execEvent.ID = id
		execEvent.TriggerType = "GitOps"
		execEvent.Metadata["git_commit_sha"] = sha
		execEvent.Timestamp = time.Now().Add(-2*time.Hour + time.Duration(i)*30*time.Minute)

		err = store.SaveChangeEvent(context.Background(), execEvent)
		require.NoError(t, err)

		engine := correlator.NewEngine(store, mockMetrics, cfg)
		analysis, err := engine.AnalyzeImpact(context.Background(), execEvent)
		require.NoError(t, err)

		shared.AssertOrphaned(t, analysis, false)
	}
}

func TestCIEvent_BothSHAAndImageTag(t *testing.T) {
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := &config.Config{}
	cfg.Analysis.IntentExecutionCorrelationDur = 2 * time.Hour

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	sha := "dual-meta-sha"
	imageTag := "v2.0.0"

	// CI event has both SHA and image_tag
	ciEvent := shared.SampleCIEvent()
	ciEvent.ID = "ci-dual-meta"
	ciEvent.Metadata["git_commit_sha"] = sha
	ciEvent.Metadata["image_tag"] = imageTag
	ciEvent.Timestamp = time.Now().Add(-3 * time.Hour)

	err = store.SaveChangeEvent(context.Background(), ciEvent)
	require.NoError(t, err)

	// Execution event matching by SHA only
	execEvent := shared.SampleChangeEvent()
	execEvent.ID = "exec-sha-only"
	execEvent.TriggerType = "GitOps"
	execEvent.Metadata = map[string]string{
		"git_commit_sha": sha,
	}
	execEvent.Timestamp = time.Now().Add(-2 * time.Hour)

	err = store.SaveChangeEvent(context.Background(), execEvent)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)
	analysis, err := engine.AnalyzeImpact(context.Background(), execEvent)
	require.NoError(t, err)

	shared.AssertOrphaned(t, analysis, false)
}

func TestCIEventArrivesAfterExecution(t *testing.T) {
	// Edge case: execution event is saved first, CI event saved later.
	// The correlator looks backward in time for CI events, so as long as the CI event
	// timestamp is before the execution event timestamp and within the correlation window,
	// it should link regardless of save order.
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := &config.Config{}
	cfg.Analysis.IntentExecutionCorrelationDur = 2 * time.Hour

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	sha := "late-arrival-sha-456"

	// Save execution event first
	execEvent := shared.SampleChangeEvent()
	execEvent.ID = "exec-before-ci"
	execEvent.TriggerType = "GitOps"
	execEvent.Metadata = map[string]string{
		"git_commit_sha": sha,
	}
	execEvent.Timestamp = time.Now().Add(-2 * time.Hour)

	err = store.SaveChangeEvent(context.Background(), execEvent)
	require.NoError(t, err)

	// Now save CI event (timestamp before execution, but saved after)
	ciEvent := shared.SampleCIEvent()
	ciEvent.ID = "ci-late-arrival"
	ciEvent.Metadata["git_commit_sha"] = sha
	ciEvent.Timestamp = time.Now().Add(-3 * time.Hour) // CI happened before exec

	err = store.SaveChangeEvent(context.Background(), ciEvent)
	require.NoError(t, err)

	// Analyze the execution event — should find the CI event
	engine := correlator.NewEngine(store, mockMetrics, cfg)
	analysis, err := engine.AnalyzeImpact(context.Background(), execEvent)
	require.NoError(t, err)

	shared.AssertOrphaned(t, analysis, false)
}
