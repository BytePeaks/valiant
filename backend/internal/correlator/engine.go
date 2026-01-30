package correlator

import (
	"context"
	"math"
	"time"
	"valiant/internal/domain"
	"valiant/internal/metrics"
	"valiant/internal/storage"
)

const (
	// Metric Weights
	weightErrorRate        = 0.4
	weightLatencyP95       = 0.3
	weightCPUSaturation    = 0.1
	weightMemorySaturation = 0.1
	weightRPS              = 0.1

	// Impact Thresholds
	thresholdHigh   = 0.7
	thresholdMedium = 0.4
	thresholdLow    = 0.1
)

type Engine struct {
	storage storage.Storage
	metrics metrics.MetricsProvider
}

func NewEngine(s storage.Storage, m metrics.MetricsProvider) *Engine {
	return &Engine{
		storage: s,
		metrics: m,
	}
}

func (e *Engine) AnalyzeImpact(ctx context.Context, event domain.ChangeEvent) (domain.ImpactAnalysis, error) {
	// 1. Define time windows relative to the change event.
	// Baseline: 30m to 5m before event
	baselineStart := event.Timestamp.Add(-30 * time.Minute)
	baselineEnd := event.Timestamp.Add(-5 * time.Minute)

	// Impact: 5m to 30m after event
	impactStart := event.Timestamp.Add(5 * time.Minute)
	impactEnd := event.Timestamp.Add(30 * time.Minute)

	// 2. Query Prometheus for average metrics in each window.
	baselineMetrics, err := e.metrics.GetAverageMetrics(ctx, event.AffectedServices, baselineStart, baselineEnd)
	if err != nil {
		return domain.ImpactAnalysis{}, err
	}

	impactMetrics, err := e.metrics.GetAverageMetrics(ctx, event.AffectedServices, impactStart, impactEnd)
	if err != nil {
		return domain.ImpactAnalysis{}, err
	}

	// 3. Calculate percentage deltas for each metric.
	deltas := calculateDeltas(baselineMetrics, impactMetrics)

	// 4. Normalize deltas into a 0-1 score and compute weighted sum.
	impactScore := calculateImpactScore(deltas)

	// 5. Classify impact level.
	impactLevel := classifyImpactLevel(impactScore)

	return domain.ImpactAnalysis{
		ChangeEvent:     event,
		BaselineMetrics: baselineMetrics,
		ImpactMetrics:   impactMetrics,
		Deltas:          deltas,
		ImpactScore:     impactScore,
		ImpactLevel:     impactLevel,
		ConfidenceScore: 1.0, // Default for deterministic logic
	}, nil
}

func calculateDeltas(baseline, impact domain.MetricValues) domain.MetricValues {
	return domain.MetricValues{
		ErrorRate:        calculateDelta(baseline.ErrorRate, impact.ErrorRate),
		LatencyP95:       calculateDelta(baseline.LatencyP95, impact.LatencyP95),
		RPS:              calculateDelta(baseline.RPS, impact.RPS),
		CPUSaturation:    calculateDelta(baseline.CPUSaturation, impact.CPUSaturation),
		MemorySaturation: calculateDelta(baseline.MemorySaturation, impact.MemorySaturation),
	}
}

func calculateDelta(baseline, impact float64) float64 {
	if baseline == 0 {
		if impact > 0 {
			return 1.0 // Treat as 100% increase if starting from 0
		}
		return 0.0
	}
	return (impact - baseline) / baseline
}

func calculateImpactScore(deltas domain.MetricValues) float64 {
	// Normalize scores (cap at 1.0, min 0.0)
	// For error, latency, cpu, mem: increase is bad.
	// For RPS: decrease is bad.

	normError := math.Max(0, math.Min(deltas.ErrorRate/2.0, 1.0))
	normLatency := math.Max(0, math.Min(deltas.LatencyP95/2.0, 1.0))
	normCPU := math.Max(0, math.Min(deltas.CPUSaturation/2.0, 1.0))
	normMem := math.Max(0, math.Min(deltas.MemorySaturation/2.0, 1.0))

	// RPS: We care about drops. A drop of 100% (-1.0) should be score 1.0.
	// Delta is (impact - baseline) / baseline.
	// If baseline 100, impact 0 -> delta = -1.0.  -1.0 * -1 = 1.0.
	normRPS := math.Max(0, math.Min(deltas.RPS*-1.0, 1.0))

	totalScore := (normError * weightErrorRate) +
		(normLatency * weightLatencyP95) +
		(normCPU * weightCPUSaturation) +
		(normMem * weightMemorySaturation) +
		(normRPS * weightRPS)

	return totalScore
}

func classifyImpactLevel(score float64) string {
	if score >= thresholdHigh {
		return "HIGH"
	}
	if score >= thresholdMedium {
		return "MEDIUM"
	}
	if score >= thresholdLow {
		return "LOW"
	}
	return "NONE"
}
