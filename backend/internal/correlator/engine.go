package correlator

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"
	"valiant/internal/config"
	"valiant/internal/domain"
	"valiant/internal/metrics"
	"valiant/internal/storage"

	"github.com/google/uuid"
)

const (
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

	// 2. Intent/Execution Linking - creates persistent links and determines orphan status
	var allLinks []domain.EventLink

	// Create intent-execution links (SHA/tag matches to CI events)
	intentLinks, err := e.CreateIntentExecutionLinks(ctx, event)
	if err != nil {
		return domain.ImpactAnalysis{}, fmt.Errorf("failed to create intent-execution links: %w", err)
	}
	allLinks = append(allLinks, intentLinks...)

	// Create config trigger links (config changes before rollouts)
	configLinks, err := e.CreateConfigTriggerLinks(ctx, event)
	if err != nil {
		return domain.ImpactAnalysis{}, fmt.Errorf("failed to create config trigger links: %w", err)
	}
	allLinks = append(allLinks, configLinks...)

	analysis.Links = allLinks

	// Backwards-compatible orphan detection: orphaned if no intent-execution links exist
	isExecutionEvent := event.TriggerType == "GitOps" || event.TriggerType == "manual"
	if isExecutionEvent {
		hasIntentLink := false
		for _, link := range intentLinks {
			if link.LinkType == "sha_match" || link.LinkType == "image_tag_match" {
				hasIntentLink = true
				break
			}
		}
		analysis.IsOrphaned = !hasIntentLink
	}

	// Populate blast radius from the event if present
	if event.BlastRadius != nil {
		analysis.BlastRadius = event.BlastRadius
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
	impactScore := calculateImpactScore(deltas, e.config.Analysis.WeightsBuiltIn, e.config.Analysis.WeightsCustom)

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

// CreateIntentExecutionLinks creates persistent links between execution events and CI events
// based on git_commit_sha and image_tag metadata matches.
func (e *Engine) CreateIntentExecutionLinks(ctx context.Context, event domain.ChangeEvent) ([]domain.EventLink, error) {
	// Only process execution events (GitOps or manual deployments)
	isExecutionEvent := event.TriggerType == "GitOps" || event.TriggerType == "manual"
	if !isExecutionEvent {
		return nil, nil
	}

	// Check if links already exist (idempotent)
	existingLinks, err := e.storage.GetEventLinksByExecutionID(ctx, event.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing links: %w", err)
	}

	// Filter for SHA/tag links
	var intentLinks []domain.EventLink
	for _, link := range existingLinks {
		if link.LinkType == "sha_match" || link.LinkType == "image_tag_match" {
			intentLinks = append(intentLinks, link)
		}
	}
	if len(intentLinks) > 0 {
		return intentLinks, nil
	}

	// Build metadata to search for
	metadataToLink := make(map[string]string)
	if sha, ok := event.Metadata["git_commit_sha"]; ok && sha != "" {
		metadataToLink["git_commit_sha"] = sha
	}
	if tag, ok := event.Metadata["image_tag"]; ok && tag != "" {
		metadataToLink["image_tag"] = tag
	}

	if len(metadataToLink) == 0 {
		return nil, nil // No linking metadata available
	}

	// Search for CI events within correlation window
	from := event.Timestamp.Add(-e.config.Analysis.IntentExecutionCorrelationDur)
	to := event.Timestamp

	ciEvents, _, err := e.storage.GetChangeEvents(ctx, map[string]interface{}{
		"trigger_type":     "CI",
		"from_timestamp":   from,
		"to_timestamp":     to,
		"metadata_has_any": metadataToLink,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search for CI events: %w", err)
	}

	// Create links for matching CI events
	var links []domain.EventLink
	now := time.Now().UTC()

	for _, ciEvent := range ciEvents {
		// Check for SHA match
		if sha, ok := event.Metadata["git_commit_sha"]; ok && sha != "" {
			if ciSha, ok := ciEvent.Metadata["git_commit_sha"]; ok && ciSha == sha {
				link := domain.EventLink{
					ID:               uuid.New().String(),
					IntentEventID:    ciEvent.ID,
					ExecutionEventID: event.ID,
					LinkType:         "sha_match",
					Confidence:       1.0,
					CreatedAt:        now,
					Metadata: map[string]string{
						"git_commit_sha": sha,
					},
				}
				if err := e.storage.SaveEventLink(ctx, link); err != nil {
					return nil, fmt.Errorf("failed to save SHA link: %w", err)
				}
				links = append(links, link)
			}
		}

		// Check for image tag match (separate link)
		if tag, ok := event.Metadata["image_tag"]; ok && tag != "" {
			if ciTag, ok := ciEvent.Metadata["image_tag"]; ok && ciTag == tag {
				// Avoid duplicate link if same CI event already linked by SHA
				alreadyLinked := false
				for _, existing := range links {
					if existing.IntentEventID == ciEvent.ID {
						alreadyLinked = true
						break
					}
				}
				if !alreadyLinked {
					link := domain.EventLink{
						ID:               uuid.New().String(),
						IntentEventID:    ciEvent.ID,
						ExecutionEventID: event.ID,
						LinkType:         "image_tag_match",
						Confidence:       0.9,
						CreatedAt:        now,
						Metadata: map[string]string{
							"image_tag": tag,
						},
					}
					if err := e.storage.SaveEventLink(ctx, link); err != nil {
						return nil, fmt.Errorf("failed to save tag link: %w", err)
					}
					links = append(links, link)
				}
			}
		}
	}

	return links, nil
}

// CreateConfigTriggerLinks creates links between deployment/statefulset rollouts and
// recent ConfigMap/Secret changes that may have triggered the rollout.
func (e *Engine) CreateConfigTriggerLinks(ctx context.Context, event domain.ChangeEvent) ([]domain.EventLink, error) {
	// Only process rollout events
	if event.ChangeType != "deployment_rollout" && event.ChangeType != "statefulset_rollout" {
		return nil, nil
	}

	// Check if config trigger links already exist
	existingLinks, err := e.storage.GetEventLinksByExecutionID(ctx, event.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing config links: %w", err)
	}

	var configLinks []domain.EventLink
	for _, link := range existingLinks {
		if link.LinkType == "config_trigger" {
			configLinks = append(configLinks, link)
		}
	}
	if len(configLinks) > 0 {
		return configLinks, nil
	}

	// Search for config changes before the rollout timestamp
	configTriggerDur := e.config.Analysis.ConfigTriggerDur
	if configTriggerDur == 0 {
		configTriggerDur = 15 * time.Minute // Default fallback
	}

	// Deduplicate config events by ID (most recent per resource)
	seenConfigEvents := make(map[string]domain.ChangeEvent)

	for _, service := range event.AffectedServices {
		configEvents, err := e.storage.GetRecentConfigChangeEvents(ctx, service, event.Timestamp, configTriggerDur)
		if err != nil {
			return nil, fmt.Errorf("failed to get recent config changes for %s: %w", service, err)
		}

		for _, configEvent := range configEvents {
			// Keep only the most recent per event ID (already sorted by timestamp DESC)
			if _, exists := seenConfigEvents[configEvent.ID]; !exists {
				seenConfigEvents[configEvent.ID] = configEvent
			}
		}
	}

	// Create links with proximity-based confidence
	var links []domain.EventLink
	now := time.Now().UTC()

	for _, configEvent := range seenConfigEvents {
		// Calculate proximity ratio: 1.0 at rollout time, 0.0 at window edge
		timeDelta := event.Timestamp.Sub(configEvent.Timestamp)
		proximityRatio := 1.0 - (float64(timeDelta) / float64(configTriggerDur))
		if proximityRatio < 0 {
			proximityRatio = 0
		}
		if proximityRatio > 1 {
			proximityRatio = 1
		}

		// Confidence = 0.7 + (0.2 * proximity)
		confidence := 0.7 + (0.2 * proximityRatio)

		link := domain.EventLink{
			ID:               uuid.New().String(),
			IntentEventID:    configEvent.ID,
			ExecutionEventID: event.ID,
			LinkType:         "config_trigger",
			Confidence:       confidence,
			CreatedAt:        now,
			Metadata: map[string]string{
				"config_event_type": configEvent.ChangeType,
				"time_delta_sec":    strconv.FormatInt(int64(timeDelta.Seconds()), 10),
			},
		}

		if err := e.storage.SaveEventLink(ctx, link); err != nil {
			return nil, fmt.Errorf("failed to save config trigger link: %w", err)
		}
		links = append(links, link)
	}

	return links, nil
}

// changeTypeWeights maps change types to risk weights.
// Full image deployments carry the highest risk; config changes are lower.
var changeTypeWeights = map[string]float64{
	"deployment_rollout":   1.0,
	"statefulset_rollout":  1.0,
	"build_success":        0.8,
	"tag_created":          0.7,
	"release_published":    0.7,
	"branch_merged":        0.6,
	"configmap_update":     0.5,
	"secret_update":        0.5,
}

// RankChanges ranks concurrent changes for a service within a time range
// by their likelihood of causing degradation.
func (e *Engine) RankChanges(ctx context.Context, service string, from, to time.Time) ([]domain.RankedChange, error) {
	events, _, err := e.storage.GetChangeEvents(ctx, map[string]interface{}{
		"from_timestamp": from,
		"to_timestamp":   to,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch events for ranking: %w", err)
	}

	if len(events) == 0 {
		return []domain.RankedChange{}, nil
	}

	// Midpoint of the query window — changes closer to this are more suspicious
	midpoint := from.Add(to.Sub(from) / 2)
	maxDistance := to.Sub(from) / 2
	if maxDistance == 0 {
		maxDistance = time.Minute // avoid division by zero
	}

	var ranked []domain.RankedChange
	for _, event := range events {
		// Get or compute the impact analysis
		analysis, err := e.AnalyzeImpact(ctx, event)
		if err == ErrImpactWindowNotClosed {
			// Include pending events with zero impact score
			analysis.ImpactLevel = "PENDING"
		} else if err != nil {
			continue // skip events that fail analysis
		}

		// Temporal proximity: 1.0 = right at midpoint, 0.0 = at edge of window
		distance := math.Abs(float64(event.Timestamp.Sub(midpoint)))
		temporal := 1.0 - math.Min(distance/float64(maxDistance), 1.0)

		// Change type weight
		typeWeight := 0.5 // default for unknown types
		if w, ok := changeTypeWeights[event.ChangeType]; ok {
			typeWeight = w
		}

		// Service scope: 1.0 if the service is directly affected, 0.5 otherwise
		scope := 0.5
		for _, s := range event.AffectedServices {
			if s == service {
				scope = 1.0
				break
			}
		}

		// Composite likelihood: weighted combination of all factors
		// impact_score (0.5) + temporal (0.2) + change_type (0.2) + scope (0.1)
		likelihood := (analysis.ImpactScore * 0.5) +
			(temporal * 0.2) +
			(typeWeight * 0.2) +
			(scope * 0.1)

		ranked = append(ranked, domain.RankedChange{
			Analysis:          analysis,
			LikelihoodScore:   likelihood,
			TemporalProximity: temporal,
			ChangeTypeWeight:  typeWeight,
			ServiceScope:      scope,
		})
	}

	// Sort by likelihood descending
	sortRankedChanges(ranked)

	// Assign ranks
	for i := range ranked {
		ranked[i].Rank = i + 1
	}

	return ranked, nil
}

// sortRankedChanges sorts ranked changes by likelihood score descending.
func sortRankedChanges(ranked []domain.RankedChange) {
	for i := 1; i < len(ranked); i++ {
		for j := i; j > 0 && ranked[j].LikelihoodScore > ranked[j-1].LikelihoodScore; j-- {
			ranked[j], ranked[j-1] = ranked[j-1], ranked[j]
		}
	}
}

func calculateDeltas(baseline, impact domain.MetricValues) domain.MetricValues {
	deltas := domain.MetricValues{
		ErrorRate:  calculateDelta(baseline.ErrorRate, impact.ErrorRate),
		LatencyP95: calculateDelta(baseline.LatencyP95, impact.LatencyP95),
		RPS:        calculateDelta(baseline.RPS, impact.RPS),
		CPU:        calculateDelta(baseline.CPU, impact.CPU),
		Memory:     calculateDelta(baseline.Memory, impact.Memory),
		AdditionalMetrics: make(map[string]float64), // Initialize the map
	}

	for key, baselineVal := range baseline.AdditionalMetrics {
		if impactVal, ok := impact.AdditionalMetrics[key]; ok {
			deltas.AdditionalMetrics[key] = calculateDelta(baselineVal, impactVal)
		} else {
			// If an additional metric is in baseline but not in impact, treat impact as 0 for delta calculation
			deltas.AdditionalMetrics[key] = calculateDelta(baselineVal, 0)
		}
	}
	// Also consider metrics that might be in impact but not baseline (newly introduced metrics, though less likely for this use case)
	for key, impactVal := range impact.AdditionalMetrics {
		if _, ok := baseline.AdditionalMetrics[key]; !ok {
			// If an additional metric is in impact but not in baseline, treat baseline as 0 for delta calculation
			deltas.AdditionalMetrics[key] = calculateDelta(0, impactVal)
		}
	}

	return deltas
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

func calculateImpactScore(deltas domain.MetricValues, weightsBuiltIn, weightsCustom map[string]float64) float64 {
	// 1. Combine all weights and create a map of normalized metric scores
	allWeights := make(map[string]float64)
	normalizedScores := make(map[string]float64)
	totalWeight := 0.0

	// Handle built-in metrics
	if w, ok := weightsBuiltIn["error_rate"]; ok {
		allWeights["error_rate"] = w
		totalWeight += w
		normalizedScores["error_rate"] = math.Max(0, math.Min(deltas.ErrorRate/2.0, 1.0))
	}
	if w, ok := weightsBuiltIn["latency_p95_ms"]; ok {
		allWeights["latency_p95_ms"] = w
		totalWeight += w
		normalizedScores["latency_p95_ms"] = math.Max(0, math.Min(deltas.LatencyP95/2.0, 1.0))
	}
	if w, ok := weightsBuiltIn["cpu"]; ok {
		allWeights["cpu"] = w
		totalWeight += w
		normalizedScores["cpu"] = math.Max(0, math.Min(deltas.CPU/2.0, 1.0))
	}
	if w, ok := weightsBuiltIn["memory"]; ok {
		allWeights["memory"] = w
		totalWeight += w
		normalizedScores["memory"] = math.Max(0, math.Min(deltas.Memory/2.0, 1.0))
	}
	if w, ok := weightsBuiltIn["rps"]; ok {
		allWeights["rps"] = w
		totalWeight += w
		// RPS: We care about drops. A drop of 100% (-1.0) should be score 1.0.
		normalizedScores["rps"] = math.Max(0, math.Min(deltas.RPS*-1.0, 1.0))
	}

	// Handle custom metrics
	for name, delta := range deltas.AdditionalMetrics {
		if w, ok := weightsCustom[name]; ok {
			allWeights[name] = w
			totalWeight += w
			// Assuming higher delta is worse for custom metrics for now
			normalizedScores[name] = math.Max(0, math.Min(delta/2.0, 1.0))
		}
	}

	// 2. Normalize weights if total is not 0
	if totalWeight > 0 {
		for name := range allWeights {
			allWeights[name] /= totalWeight
		}
	}

	// 3. Calculate final weighted score
	totalScore := 0.0
	for name, score := range normalizedScores {
		if weight, ok := allWeights[name]; ok {
			totalScore += score * weight
		}
	}

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
