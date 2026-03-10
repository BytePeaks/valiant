package sha_match

import (
	"context"
	"testing"
	"time"
	"valiant/internal/correlator"
	"valiant/internal/storage"
	"valiant/tests/common"
	"valiant/tests/event-links/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShaMatchCreatesLink(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	cfg.Analysis.ConfigTriggerWindow = "15m"
	cfg.Analysis.ConfigTriggerDur = 15 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	const matchingSha = "abc123def456"

	// CI event
	ciEvent := common.SampleCIEvent()
	ciEvent.Timestamp = time.Now().Add(-1 * time.Hour)
	ciEvent.Metadata["git_commit_sha"] = matchingSha

	// Execution event with same SHA
	execEvent := common.SampleChangeEvent()
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
	require.NotEmpty(t, analysis.Links, "Expected at least one link")
	shared.AssertLinkExists(t, analysis.Links, "sha_match", 1.0, 0.01)
	shared.AssertOrphaned(t, analysis, false)
}

func TestNoShaMatch_NoLink(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	cfg.Analysis.ConfigTriggerWindow = "15m"
	cfg.Analysis.ConfigTriggerDur = 15 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	// CI event with different SHA
	ciEvent := common.SampleCIEvent()
	ciEvent.Timestamp = time.Now().Add(-1 * time.Hour)
	ciEvent.Metadata["git_commit_sha"] = "different_sha"

	// Execution event
	execEvent := common.SampleChangeEvent()
	execEvent.Timestamp = time.Now().Add(-50 * time.Minute)
	execEndTime := execEvent.Timestamp.Add(5 * time.Minute)
	execEvent.EndTime = &execEndTime
	execEvent.Metadata["git_commit_sha"] = "abc123def456"

	err = store.SaveChangeEvent(context.Background(), ciEvent)
	require.NoError(t, err)
	err = store.SaveChangeEvent(context.Background(), execEvent)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)

	// ACT
	analysis, err := engine.AnalyzeImpact(context.Background(), execEvent)
	require.NoError(t, err)

	// ASSERT
	shared.AssertNoLinkOfType(t, analysis.Links, "sha_match")
	shared.AssertOrphaned(t, analysis, true)
}

func TestShaMatchOutsideWindow_NoLink(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	cfg.Analysis.IntentExecutionCorrelationDur = 30 * time.Minute // Short window
	cfg.Analysis.ConfigTriggerWindow = "15m"
	cfg.Analysis.ConfigTriggerDur = 15 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	const matchingSha = "abc123def456"

	// CI event far outside the correlation window
	ciEvent := common.SampleCIEvent()
	ciEvent.Timestamp = time.Now().Add(-5 * time.Hour)
	ciEvent.Metadata["git_commit_sha"] = matchingSha

	// Execution event
	execEvent := common.SampleChangeEvent()
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
	shared.AssertNoLinkOfType(t, analysis.Links, "sha_match")
	shared.AssertOrphaned(t, analysis, true)
}

func TestImageTagMatchCreatesLink(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	cfg.Analysis.ConfigTriggerWindow = "15m"
	cfg.Analysis.ConfigTriggerDur = 15 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	const matchingTag = "v1.2.3"

	// CI event with image tag only (no SHA)
	ciEvent := common.SampleCIEvent()
	ciEvent.ID = "ci-tag-only"
	ciEvent.Timestamp = time.Now().Add(-1 * time.Hour)
	ciEvent.Metadata = map[string]string{
		"image_tag": matchingTag,
	}

	// Execution event with same image tag (no SHA)
	execEvent := common.SampleChangeEvent()
	execEvent.ID = "exec-tag-only"
	execEvent.Timestamp = time.Now().Add(-50 * time.Minute)
	execEndTime := execEvent.Timestamp.Add(5 * time.Minute)
	execEvent.EndTime = &execEndTime
	execEvent.Metadata = map[string]string{
		"image_tag": matchingTag,
		"namespace": "production",
	}

	err = store.SaveChangeEvent(context.Background(), ciEvent)
	require.NoError(t, err)
	err = store.SaveChangeEvent(context.Background(), execEvent)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)

	// ACT
	analysis, err := engine.AnalyzeImpact(context.Background(), execEvent)
	require.NoError(t, err)

	// ASSERT
	require.NotEmpty(t, analysis.Links)
	shared.AssertLinkExists(t, analysis.Links, "image_tag_match", 0.9, 0.01)
	shared.AssertOrphaned(t, analysis, false)
}

func TestBothShaAndTagCreatesSingleLink(t *testing.T) {
	// When both SHA and tag match on the same CI event, SHA match takes priority
	// and only one link is created (deduplicated by CI event ID)
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	cfg.Analysis.ConfigTriggerWindow = "15m"
	cfg.Analysis.ConfigTriggerDur = 15 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	const matchingSha = "abc123"
	const matchingTag = "v1.0.0"

	ciEvent := common.SampleCIEvent()
	ciEvent.Timestamp = time.Now().Add(-1 * time.Hour)
	ciEvent.Metadata["git_commit_sha"] = matchingSha
	ciEvent.Metadata["image_tag"] = matchingTag

	execEvent := common.SampleChangeEvent()
	execEvent.Timestamp = time.Now().Add(-50 * time.Minute)
	execEndTime := execEvent.Timestamp.Add(5 * time.Minute)
	execEvent.EndTime = &execEndTime
	execEvent.Metadata["git_commit_sha"] = matchingSha
	execEvent.Metadata["image_tag"] = matchingTag

	err = store.SaveChangeEvent(context.Background(), ciEvent)
	require.NoError(t, err)
	err = store.SaveChangeEvent(context.Background(), execEvent)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)

	analysis, err := engine.AnalyzeImpact(context.Background(), execEvent)
	require.NoError(t, err)

	// SHA match from the same CI event deduplicates the tag match
	shaLinks := 0
	tagLinks := 0
	for _, link := range analysis.Links {
		if link.LinkType == "sha_match" {
			shaLinks++
		}
		if link.LinkType == "image_tag_match" {
			tagLinks++
		}
	}
	assert.Equal(t, 1, shaLinks, "Expected exactly one SHA link")
	assert.Equal(t, 0, tagLinks, "Tag link should be deduplicated when SHA matches same CI event")
	shared.AssertOrphaned(t, analysis, false)
}
