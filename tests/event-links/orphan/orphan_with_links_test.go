package orphan

import (
	"context"
	"testing"
	"time"
	"valiant/internal/correlator"
	"valiant/internal/storage"
	"valiant/tests/common"
	"valiant/tests/event-links/shared"

	"github.com/stretchr/testify/require"
)

func TestEventWithLinks_NotOrphaned(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	cfg.Analysis.ConfigTriggerWindow = "15m"
	cfg.Analysis.ConfigTriggerDur = 15 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	const matchingSha = "sha-for-orphan-test"

	ciEvent := common.SampleCIEvent()
	ciEvent.ID = "ci-orphan-test"
	ciEvent.Timestamp = time.Now().Add(-1 * time.Hour)
	ciEvent.Metadata["git_commit_sha"] = matchingSha

	execEvent := common.SampleChangeEvent()
	execEvent.ID = "exec-orphan-test"
	execEvent.Timestamp = time.Now().Add(-50 * time.Minute)
	execEndTime := execEvent.Timestamp.Add(5 * time.Minute)
	execEvent.EndTime = &execEndTime
	execEvent.Metadata["git_commit_sha"] = matchingSha

	err = store.SaveChangeEvent(context.Background(), ciEvent)
	require.NoError(t, err)
	err = store.SaveChangeEvent(context.Background(), execEvent)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)

	// ACT
	analysis, err := engine.AnalyzeImpact(context.Background(), execEvent)
	require.NoError(t, err)

	// ASSERT
	shared.AssertOrphaned(t, analysis, false)
	require.NotEmpty(t, analysis.Links, "Should have at least one link")
}

func TestEventWithoutLinks_IsOrphaned(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	cfg.Analysis.ConfigTriggerWindow = "15m"
	cfg.Analysis.ConfigTriggerDur = 15 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	// Execution event with no matching CI event
	execEvent := common.SampleChangeEvent()
	execEvent.ID = "exec-orphan-no-link"
	execEvent.Timestamp = time.Now().Add(-50 * time.Minute)
	execEndTime := execEvent.Timestamp.Add(5 * time.Minute)
	execEvent.EndTime = &execEndTime
	execEvent.Metadata["git_commit_sha"] = "unique-sha-no-match"

	err = store.SaveChangeEvent(context.Background(), execEvent)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)

	// ACT
	analysis, err := engine.AnalyzeImpact(context.Background(), execEvent)
	require.NoError(t, err)

	// ASSERT
	shared.AssertOrphaned(t, analysis, true)
	require.Empty(t, analysis.Links, "Should have no links")
}
