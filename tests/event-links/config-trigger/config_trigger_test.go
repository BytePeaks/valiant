package config_trigger

import (
	"context"
	"testing"
	"time"
	"valiant/internal/correlator"
	"valiant/internal/domain"
	"valiant/internal/storage"
	"valiant/tests/common"
	"valiant/tests/event-links/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigTriggerLink_WithinWindow(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	cfg.Analysis.ConfigTriggerWindow = "15m"
	cfg.Analysis.ConfigTriggerDur = 15 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	// Config change 5 minutes before rollout
	rolloutTimestamp := time.Now().Add(-50 * time.Minute)
	configEvent := domain.ChangeEvent{
		ID:               "config-change-1",
		Source:           "kubernetes",
		TriggerType:      "GitOps",
		ChangeType:       "configmap_update",
		Timestamp:        rolloutTimestamp.Add(-5 * time.Minute),
		AffectedServices: []string{"api-service"},
		Metadata:         map[string]string{"namespace": "production", "kind": "ConfigMap"},
		Summary:          "ConfigMap production/app-config data changed",
	}

	// Deployment rollout
	rolloutEndTime := rolloutTimestamp.Add(5 * time.Minute)
	rolloutEvent := domain.ChangeEvent{
		ID:               "rollout-1",
		Source:           "kubernetes",
		TriggerType:      "GitOps",
		ChangeType:       "deployment_rollout",
		Timestamp:        rolloutTimestamp,
		EndTime:          &rolloutEndTime,
		AffectedServices: []string{"api-service"},
		Metadata:         map[string]string{"namespace": "production", "git_commit_sha": "abc123"},
		Summary:          "Deployment api-service rollout completed",
	}

	err = store.SaveChangeEvent(context.Background(), configEvent)
	require.NoError(t, err)
	err = store.SaveChangeEvent(context.Background(), rolloutEvent)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)

	// ACT
	analysis, err := engine.AnalyzeImpact(context.Background(), rolloutEvent)
	require.NoError(t, err)

	// ASSERT
	shared.AssertLinkExists(t, analysis.Links, "config_trigger", 0.83, 0.1)
	// 5 min within 15 min window: proximity = 1 - (5/15) = 0.667, confidence = 0.7 + 0.2*0.667 = 0.833
}

func TestConfigTriggerLink_AfterRollout_NoLink(t *testing.T) {
	// Config change AFTER the rollout should not create a link
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	cfg.Analysis.ConfigTriggerWindow = "15m"
	cfg.Analysis.ConfigTriggerDur = 15 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	rolloutTimestamp := time.Now().Add(-50 * time.Minute)

	// Config change AFTER rollout
	configEvent := domain.ChangeEvent{
		ID:               "config-after-rollout",
		Source:           "kubernetes",
		TriggerType:      "GitOps",
		ChangeType:       "configmap_update",
		Timestamp:        rolloutTimestamp.Add(10 * time.Minute), // After rollout
		AffectedServices: []string{"api-service"},
		Metadata:         map[string]string{"namespace": "production"},
		Summary:          "ConfigMap changed after rollout",
	}

	rolloutEndTime := rolloutTimestamp.Add(5 * time.Minute)
	rolloutEvent := domain.ChangeEvent{
		ID:               "rollout-after",
		Source:           "kubernetes",
		TriggerType:      "GitOps",
		ChangeType:       "deployment_rollout",
		Timestamp:        rolloutTimestamp,
		EndTime:          &rolloutEndTime,
		AffectedServices: []string{"api-service"},
		Metadata:         map[string]string{"namespace": "production"},
		Summary:          "Deployment rollout",
	}

	err = store.SaveChangeEvent(context.Background(), configEvent)
	require.NoError(t, err)
	err = store.SaveChangeEvent(context.Background(), rolloutEvent)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)

	analysis, err := engine.AnalyzeImpact(context.Background(), rolloutEvent)
	require.NoError(t, err)

	shared.AssertNoLinkOfType(t, analysis.Links, "config_trigger")
}

func TestConfigTriggerLink_OutsideWindow_NoLink(t *testing.T) {
	// Config change outside the trigger window should not create a link
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	cfg.Analysis.ConfigTriggerWindow = "15m"
	cfg.Analysis.ConfigTriggerDur = 15 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	rolloutTimestamp := time.Now().Add(-50 * time.Minute)

	// Config change 30 minutes before rollout — outside 15m window
	configEvent := domain.ChangeEvent{
		ID:               "config-old",
		Source:           "kubernetes",
		TriggerType:      "GitOps",
		ChangeType:       "configmap_update",
		Timestamp:        rolloutTimestamp.Add(-30 * time.Minute),
		AffectedServices: []string{"api-service"},
		Metadata:         map[string]string{"namespace": "production"},
		Summary:          "Old ConfigMap change",
	}

	rolloutEndTime := rolloutTimestamp.Add(5 * time.Minute)
	rolloutEvent := domain.ChangeEvent{
		ID:               "rollout-outside",
		Source:           "kubernetes",
		TriggerType:      "GitOps",
		ChangeType:       "deployment_rollout",
		Timestamp:        rolloutTimestamp,
		EndTime:          &rolloutEndTime,
		AffectedServices: []string{"api-service"},
		Metadata:         map[string]string{"namespace": "production"},
		Summary:          "Deployment rollout",
	}

	err = store.SaveChangeEvent(context.Background(), configEvent)
	require.NoError(t, err)
	err = store.SaveChangeEvent(context.Background(), rolloutEvent)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)

	analysis, err := engine.AnalyzeImpact(context.Background(), rolloutEvent)
	require.NoError(t, err)

	shared.AssertNoLinkOfType(t, analysis.Links, "config_trigger")
}

func TestConfigTriggerLink_ConfidenceScalesByProximity(t *testing.T) {
	// Config changes closer to rollout get higher confidence
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	cfg.Analysis.ConfigTriggerWindow = "15m"
	cfg.Analysis.ConfigTriggerDur = 15 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	rolloutTimestamp := time.Now().Add(-50 * time.Minute)

	// Config change right before rollout (high proximity)
	nearConfigEvent := domain.ChangeEvent{
		ID:               "config-near",
		Source:           "kubernetes",
		TriggerType:      "GitOps",
		ChangeType:       "configmap_update",
		Timestamp:        rolloutTimestamp.Add(-1 * time.Minute), // 1 min before
		AffectedServices: []string{"api-service"},
		Metadata:         map[string]string{"namespace": "production"},
		Summary:          "Near config change",
	}

	rolloutEndTime := rolloutTimestamp.Add(5 * time.Minute)
	rolloutEvent := domain.ChangeEvent{
		ID:               "rollout-proximity",
		Source:           "kubernetes",
		TriggerType:      "GitOps",
		ChangeType:       "deployment_rollout",
		Timestamp:        rolloutTimestamp,
		EndTime:          &rolloutEndTime,
		AffectedServices: []string{"api-service"},
		Metadata:         map[string]string{"namespace": "production"},
		Summary:          "Deployment rollout",
	}

	err = store.SaveChangeEvent(context.Background(), nearConfigEvent)
	require.NoError(t, err)
	err = store.SaveChangeEvent(context.Background(), rolloutEvent)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)

	analysis, err := engine.AnalyzeImpact(context.Background(), rolloutEvent)
	require.NoError(t, err)

	// 1 min within 15 min window: proximity = 1 - (1/15) = 0.933, confidence = 0.7 + 0.2*0.933 = 0.887
	require.NotEmpty(t, analysis.Links)
	for _, link := range analysis.Links {
		if link.LinkType == "config_trigger" {
			assert.Greater(t, link.Confidence, 0.85, "Near config change should have high confidence")
			assert.LessOrEqual(t, link.Confidence, 0.9, "Confidence should not exceed 0.9")
			return
		}
	}
	t.Error("Expected config_trigger link not found")
}

func TestConfigTriggerLink_UnrelatedService_NoLink(t *testing.T) {
	// Config change for different service should not link
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	cfg := common.SampleConfig()
	cfg.Analysis.ConfigTriggerWindow = "15m"
	cfg.Analysis.ConfigTriggerDur = 15 * time.Minute

	store := storage.NewPostgresStorage(db, cfg)
	mockMetrics := &shared.MockMetricsProvider{}

	rolloutTimestamp := time.Now().Add(-50 * time.Minute)

	// Config change for a DIFFERENT service
	configEvent := domain.ChangeEvent{
		ID:               "config-other-svc",
		Source:           "kubernetes",
		TriggerType:      "GitOps",
		ChangeType:       "configmap_update",
		Timestamp:        rolloutTimestamp.Add(-5 * time.Minute),
		AffectedServices: []string{"other-service"}, // Different service
		Metadata:         map[string]string{"namespace": "production"},
		Summary:          "ConfigMap for other service",
	}

	rolloutEndTime := rolloutTimestamp.Add(5 * time.Minute)
	rolloutEvent := domain.ChangeEvent{
		ID:               "rollout-unrelated",
		Source:           "kubernetes",
		TriggerType:      "GitOps",
		ChangeType:       "deployment_rollout",
		Timestamp:        rolloutTimestamp,
		EndTime:          &rolloutEndTime,
		AffectedServices: []string{"api-service"},
		Metadata:         map[string]string{"namespace": "production"},
		Summary:          "Deployment rollout",
	}

	err = store.SaveChangeEvent(context.Background(), configEvent)
	require.NoError(t, err)
	err = store.SaveChangeEvent(context.Background(), rolloutEvent)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)

	analysis, err := engine.AnalyzeImpact(context.Background(), rolloutEvent)
	require.NoError(t, err)

	shared.AssertNoLinkOfType(t, analysis.Links, "config_trigger")
}
