package metrics

import (
	"context"
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

func TestAdditionalMetrics_ScoredWithCoreMetrics(t *testing.T) {
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	store := storage.NewPostgresStorage(db)
	mockMetrics := shared.NewMockMetricsProvider()

	cfg := &config.Config{}
	cfg.Analysis.BaselineDur = 30 * time.Minute
	cfg.Analysis.ImpactDur = 30 * time.Minute
	cfg.Prometheus.AdditionalMetrics = []config.PrometheusMetric{
		{Name: "custom_error_count", Query: "sum(rate(custom_errors_total[5m]))"},
	}

	eventTimestamp := time.Now().Add(-2 * time.Hour)
	endTime := time.Now().Add(-90 * time.Minute)

	event := shared.SampleChangeEvent()
	event.ID = "test-additional-1"
	event.Timestamp = eventTimestamp
	event.EndTime = &endTime

	baselineStart := eventTimestamp.Add(-cfg.Analysis.BaselineDur)
	baselineEnd := eventTimestamp.Add(-5 * time.Minute)
	impactStart := endTime.Add(5 * time.Minute)
	impactEnd := impactStart.Add(cfg.Analysis.ImpactDur)

	baseline := domain.MetricValues{
		ErrorRate: 0.01, LatencyP95: 100, RPS: 100, CPU: 50, Memory: 60,
		AdditionalMetrics: map[string]float64{"custom_error_count": 5.0},
	}
	impact := domain.MetricValues{
		ErrorRate: 0.01, LatencyP95: 100, RPS: 100, CPU: 50, Memory: 60,
		AdditionalMetrics: map[string]float64{"custom_error_count": 15.0},
	}

	mockMetrics.SetMetrics(event.AffectedServices[0], baselineStart, baselineEnd, baseline)
	mockMetrics.SetMetrics(event.AffectedServices[0], impactStart, impactEnd, impact)

	err = store.SaveChangeEvent(context.Background(), event)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)
	analysis, err := engine.AnalyzeImpact(context.Background(), event)
	require.NoError(t, err)

	// Core metrics are identical, so core impact score should be 0
	assert.Equal(t, "NONE", analysis.ImpactLevel, "Core metrics didn't change, so impact should be NONE")
	assert.Equal(t, 0.0, analysis.ImpactScore, "Core metrics identical => score 0")

	// Additional metrics should still show deltas
	assert.Equal(t, 2.0, analysis.Deltas.AdditionalMetrics["custom_error_count"],
		"Expected 200% increase delta for custom_error_count: (15-5)/5 = 2.0")
	assert.Equal(t, 5.0, analysis.BaselineMetrics.AdditionalMetrics["custom_error_count"])
	assert.Equal(t, 15.0, analysis.ImpactMetrics.AdditionalMetrics["custom_error_count"])
}

func TestAdditionalMetrics_MissingInImpact(t *testing.T) {
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	store := storage.NewPostgresStorage(db)
	mockMetrics := shared.NewMockMetricsProvider()

	cfg := &config.Config{}
	cfg.Analysis.BaselineDur = 30 * time.Minute
	cfg.Analysis.ImpactDur = 30 * time.Minute

	eventTimestamp := time.Now().Add(-2 * time.Hour)
	endTime := time.Now().Add(-90 * time.Minute)

	event := shared.SampleChangeEvent()
	event.ID = "test-additional-missing"
	event.Timestamp = eventTimestamp
	event.EndTime = &endTime

	baselineStart := eventTimestamp.Add(-cfg.Analysis.BaselineDur)
	baselineEnd := eventTimestamp.Add(-5 * time.Minute)
	impactStart := endTime.Add(5 * time.Minute)
	impactEnd := impactStart.Add(cfg.Analysis.ImpactDur)

	// Baseline has custom metric, impact does not
	baseline := domain.MetricValues{
		ErrorRate: 0.01, LatencyP95: 100, RPS: 100,
		AdditionalMetrics: map[string]float64{"vanishing_metric": 10.0},
	}
	impact := domain.MetricValues{
		ErrorRate: 0.01, LatencyP95: 100, RPS: 100,
		AdditionalMetrics: map[string]float64{},
	}

	mockMetrics.SetMetrics(event.AffectedServices[0], baselineStart, baselineEnd, baseline)
	mockMetrics.SetMetrics(event.AffectedServices[0], impactStart, impactEnd, impact)

	err = store.SaveChangeEvent(context.Background(), event)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)
	analysis, err := engine.AnalyzeImpact(context.Background(), event)
	require.NoError(t, err)

	// When metric disappears from impact (baseline=10, impact=0), delta should be -1.0
	assert.Equal(t, -1.0, analysis.Deltas.AdditionalMetrics["vanishing_metric"],
		"Missing impact metric treated as 0: (0-10)/10 = -1.0")
}

func TestAdditionalMetrics_NewInImpact(t *testing.T) {
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	store := storage.NewPostgresStorage(db)
	mockMetrics := shared.NewMockMetricsProvider()

	cfg := &config.Config{}
	cfg.Analysis.BaselineDur = 30 * time.Minute
	cfg.Analysis.ImpactDur = 30 * time.Minute

	eventTimestamp := time.Now().Add(-2 * time.Hour)
	endTime := time.Now().Add(-90 * time.Minute)

	event := shared.SampleChangeEvent()
	event.ID = "test-additional-new"
	event.Timestamp = eventTimestamp
	event.EndTime = &endTime

	baselineStart := eventTimestamp.Add(-cfg.Analysis.BaselineDur)
	baselineEnd := eventTimestamp.Add(-5 * time.Minute)
	impactStart := endTime.Add(5 * time.Minute)
	impactEnd := impactStart.Add(cfg.Analysis.ImpactDur)

	// Baseline has no custom metric, impact introduces one
	baseline := domain.MetricValues{
		ErrorRate: 0.01, LatencyP95: 100, RPS: 100,
		AdditionalMetrics: map[string]float64{},
	}
	impact := domain.MetricValues{
		ErrorRate: 0.01, LatencyP95: 100, RPS: 100,
		AdditionalMetrics: map[string]float64{"new_metric": 5.0},
	}

	mockMetrics.SetMetrics(event.AffectedServices[0], baselineStart, baselineEnd, baseline)
	mockMetrics.SetMetrics(event.AffectedServices[0], impactStart, impactEnd, impact)

	err = store.SaveChangeEvent(context.Background(), event)
	require.NoError(t, err)

	engine := correlator.NewEngine(store, mockMetrics, cfg)
	analysis, err := engine.AnalyzeImpact(context.Background(), event)
	require.NoError(t, err)

	// When metric appears in impact (baseline=0, impact>0), delta should be 1.0 (100% increase cap)
	assert.Equal(t, 1.0, analysis.Deltas.AdditionalMetrics["new_metric"],
		"New metric in impact with zero baseline: delta should be 1.0")
}
