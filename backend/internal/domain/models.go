package domain

import "time"

// ChangeEvent is the normalized representation of any collected change.
type ContextualLink struct {
	Name string `json:"name"` // Display name for the link (e.g., "View Commit on GitHub")
	URL  string `json:"url"`  // The actual URL
}

// ChangeEvent is the normalized representation of any collected change.
type ChangeEvent struct {
	ID               string            `json:"id"`
	Source           string            `json:"source"`             // DEPRECATED: Use TriggerType. e.g. "kubernetes", "ci-cd"
	TriggerType      string            `json:"trigger_type"`       // "CI", "GitOps", "manual"
	ExecutionID      string            `json:"execution_id"`       // Unique ID of the execution (e.g. CI build ID, K8s UID)
	ChangeType       string            `json:"change_type"`        // "deployment_rollout", "configmap_update", "build_success"
	Timestamp        time.Time         `json:"timestamp"`          // Start time of the execution
	EndTime          *time.Time        `json:"end_time,omitempty"` // End time of the execution (optional)
	AffectedServices []string          `json:"affected_services"`
	Metadata         map[string]string `json:"metadata"`                   // e.g., "git_commit_sha", "image_tag"
	Summary          string            `json:"summary"`                    // Human-readable summary
	ContextualLinks  []ContextualLink  `json:"contextual_links,omitempty"` //Field for generated deep links
	Status           string            `json:"status,omitempty"`
	InvalidReason    string            `json:"invalid_reason,omitempty"`
	SkewSeconds      int               `json:"skew_seconds,omitempty"`
}

// MetricValues holds the calculated values for a specific time window.
type MetricValues struct {
	ErrorRate         float64            `json:"error_rate"`
	LatencyP95        float64            `json:"latency_p95_ms"`
	RPS               float64            `json:"rps"`
	CPU               float64            `json:"cpu"`
	Memory            float64            `json:"memory"`
	AdditionalMetrics map[string]float64 `json:"additional_metrics,omitempty"` // New field for user-defined metrics
}

// ImpactAnalysis represents the full analysis of a single change event.
type ImpactAnalysis struct {
	ChangeEvent     ChangeEvent  `json:"change_event"`
	IsOrphaned      bool         `json:"is_orphaned,omitempty"` // True if no corresponding intent event was found
	BaselineMetrics MetricValues `json:"baseline_metrics"`
	ImpactMetrics   MetricValues `json:"impact_metrics"`
	Deltas          MetricValues `json:"deltas"`           // Percentage change for each metric
	ImpactScore     float64      `json:"impact_score"`     // 0.0 to 1.0
	ImpactLevel     string       `json:"impact_level"`     // "NONE", "LOW", "MEDIUM", "HIGH"
	ConfidenceScore float64      `json:"confidence_score"` // 0.0 to 1.0
}

type MetricInfo struct {
	Name   string  `json:"name"`
	Icon   string  `json:"icon,omitempty"`
	Weight float64 `json:"weight,omitempty"`
	Type   string  `json:"type,omitempty"`   // Added for classification (builtin/custom)
	Promql string  `json:"promql,omitempty"` // For built-in display example
	Query  string  `json:"query,omitempty"`  // For custom metric raw query
}

// RankedChange represents a change event ranked by likelihood of causing degradation.
type RankedChange struct {
	Analysis          ImpactAnalysis `json:"analysis"`
	Rank              int            `json:"rank"`
	LikelihoodScore   float64        `json:"likelihood_score"`   // 0.0 to 1.0, composite ranking score
	TemporalProximity float64        `json:"temporal_proximity"` // 0.0 to 1.0, how close to the query window
	ChangeTypeWeight  float64        `json:"change_type_weight"` // 0.0 to 1.0, risk weight by change type
	ServiceScope      float64        `json:"service_scope"`      // 0.0 to 1.0, direct vs indirect
}
