package correlator

import (
	"context"
	"fmt"
	"math"
	"time"
	"valiant/internal/config"
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

var ErrImpactWindowNotClosed = fmt.Errorf("impact window has not yet closed")

type Engine struct {
	storage storage.Storage
	metrics metrics.MetricsProvider
	config  *config.Config
}

func NewEngine(s storage.Storage, m metrics.MetricsProvider, cfg *config.Config) *Engine {
	return &Engine{
		storage: s,
		metrics: m,
		config:  cfg,
	}
}

func (e *Engine) AnalyzeImpact(ctx context.Context, event domain.ChangeEvent) (domain.ImpactAnalysis, error) {
	// 1. Check if snapshot exists
	existing, err := e.storage.GetImpactAnalysisByEventID(ctx, event.ID)
	if err != nil {
		return domain.ImpactAnalysis{}, err
	}
	if existing != nil {
		// Populate the ChangeEvent as it is not stored in the snapshot table fully
		existing.ChangeEvent = event
		existing.ConfidenceScore = 1.0 // Restore default
		return *existing, nil
	}

	analysis := domain.ImpactAnalysis{
		ChangeEvent: event,
	}

	// 2. Intent/Execution Linking (formerly Orphan Detection)
	isExecutionEvent := event.TriggerType == "GitOps" || event.TriggerType == "manual"
	if isExecutionEvent && (event.Metadata["git_commit_sha"] != "" || event.Metadata["image_tag"] != "") {
		// This is an execution event with linking metadata. Look for a corresponding CI event.
		from := event.Timestamp.Add(-e.config.Analysis.IntentExecutionCorrelationDur)
		to := event.Timestamp
		
		metadataToLink := make(map[string]string)
		if sha, ok := event.Metadata["git_commit_sha"]; ok && sha != "" {
			metadataToLink["git_commit_sha"] = sha
		}
		if tag, ok := event.Metadata["image_tag"]; ok && tag != "" {
			metadataToLink["image_tag"] = tag
		}

		ciEvents, err := e.storage.GetChangeEvents(ctx, map[string]interface{}{
			"trigger_type":     "CI",
			"from_timestamp":   from,
			"to_timestamp":     to,
			"metadata_has_any": metadataToLink,
		})
		if err != nil {
			return domain.ImpactAnalysis{}, fmt.Errorf("failed to check for corresponding CI event: %w", err)
		}
		if len(ciEvents) == 0 {
			analysis.IsOrphaned = true
		}
	} else if isExecutionEvent {
		// Fallback for execution events without linking metadata
		analysis.IsOrphaned = true
	}

	// 3. Define time windows relative to the change event using Config
	baselineDur := e.config.Analysis.BaselineDur
	impactDur := e.config.Analysis.ImpactDur

	// Baseline: e.g., 30m to 5m before the execution STARTED
	baselineStart := event.Timestamp.Add(-(baselineDur))
	baselineEnd := event.Timestamp.Add(-5 * time.Minute)

	// Impact Pivot: Use EndTime (rollout finished) if available, otherwise fallback to Timestamp
	impactPivot := event.Timestamp
	if event.EndTime != nil {
		impactPivot = *event.EndTime
	}

	// Impact: e.g., 5m to 35m after the execution FINISHED
	impactStart := impactPivot.Add(5 * time.Minute)
	impactEnd := impactPivot.Add(5 * time.Minute).Add(impactDur)

	// Check if impact window has closed
	if time.Now().UTC().Before(impactEnd) {
		return domain.ImpactAnalysis{
			ChangeEvent: event,
			ImpactLevel: "PENDING",
		}, ErrImpactWindowNotClosed
	}

	// 3. Query Prometheus for average metrics in each window.
	baselineMetrics, err := e.metrics.GetAverageMetrics(ctx, event.AffectedServices, baselineStart, baselineEnd)
	if err != nil {
		return domain.ImpactAnalysis{}, err
	}

	impactMetrics, err := e.metrics.GetAverageMetrics(ctx, event.AffectedServices, impactStart, impactEnd)
	if err != nil {
		return domain.ImpactAnalysis{}, err
	}

	// 4. Calculate percentage deltas for each metric.
	deltas := calculateDeltas(baselineMetrics, impactMetrics)

	// 5. Normalize deltas into a 0-1 score and compute weighted sum.
	impactScore := calculateImpactScore(deltas)

	// 6. Classify impact level.
	impactLevel := classifyImpactLevel(impactScore)

	// 7. Calculate confidence score based on data volume.
	confidenceScore := calculateConfidenceScore(baselineMetrics, impactMetrics)

	analysis.BaselineMetrics = baselineMetrics
	analysis.ImpactMetrics = impactMetrics
	analysis.Deltas = deltas
	analysis.ImpactScore = impactScore
	analysis.ImpactLevel = impactLevel
	analysis.ConfidenceScore = confidenceScore

	// 7. Save snapshot
	if err := e.storage.SaveImpactAnalysis(ctx, analysis); err != nil {
		// Log error but return analysis anyway
		fmt.Printf("Warning: Failed to save analysis snapshot: %v\n", err)
	}

	return analysis, nil
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

func calculateConfidenceScore(baseline, impact domain.MetricValues) float64 {
	// If RPS is very low, our percentage deltas are less statistically significant.
	if baseline.RPS < 1.0 && impact.RPS < 1.0 {
		return 0.5
	}
	// If we have zero metrics across the board, confidence is very low
	if baseline.RPS == 0 && impact.RPS == 0 && baseline.ErrorRate == 0 && impact.ErrorRate == 0 {
		return 0.1
	}
	return 1.0
}
