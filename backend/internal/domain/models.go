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
	IsIntent         bool              `json:"is_intent"`          // True if this event is an 'intent' (e.g., CI build)
	IsExecution      bool              `json:"is_execution"`       // True if this event is an 'execution' (e.g., K8s rollout)
	ExecutionID      string            `json:"execution_id"`       // Unique ID of the execution (e.g. CI build ID, K8s UID)
	ChangeType       string            `json:"change_type"`        // "deployment_rollout", "configmap_update", "build_success"
	Timestamp        time.Time         `json:"timestamp"`          // Start time of the execution
	EndTime          *time.Time        `json:"end_time,omitempty"` // End time of the execution (optional)
	AffectedServices []string          `json:"affected_services"`
	Metadata         map[string]string `json:"metadata"`                   // e.g., "git_commit_sha", "image_tag"
	Summary          string            `json:"summary"`                    // Human-readable summary
	ContextualLinks  []ContextualLink  `json:"contextual_links,omitempty"` //Field for generated deep links
	BlastRadius      *BlastRadius      `json:"blast_radius,omitempty"`     // Scope of affected workloads (for config changes)
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

// EventLink represents a persistent link between an intent event (CI build, config change)
// and an execution event (deployment/statefulset rollout).
type EventLink struct {
	ID               string            `json:"id"`
	IntentEventID    string            `json:"intent_event_id"`
	ExecutionEventID string            `json:"execution_event_id"`
	LinkType         string            `json:"link_type"`  // "sha_match", "image_tag_match", "config_trigger"
	Confidence       float64           `json:"confidence"` // 0.0 to 1.0
	CreatedAt        time.Time         `json:"created_at"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// BlastRadius represents the scope of workloads affected by a config change.
type BlastRadius struct {
	TotalWorkloads       int      `json:"total_workloads"`
	AffectedDeployments  []string `json:"affected_deployments"`
	AffectedStatefulSets []string `json:"affected_statefulsets"`
}

// ImpactAnalysis represents the full analysis of a single change event.
type ImpactAnalysis struct {
	ChangeEvent     ChangeEvent  `json:"change_event"`
	LinkedCIEvent   *ChangeEvent `json:"linked_ci_event,omitempty"` // Optional: Link to the CI event that caused the execution
	IsOrphaned      bool         `json:"is_orphaned,omitempty"`     // True if no corresponding intent event was found
	Links           []EventLink  `json:"links,omitempty"`           // Persistent links to intent/config events
	BlastRadius     *BlastRadius `json:"blast_radius,omitempty"`    // Scope of affected workloads (for config changes)
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

// ServiceHealth represents the current health status of a service derived from its latest impact analysis.
type ServiceHealth struct {
	Service        string     `json:"service"`
	Status         string     `json:"status"`       // "healthy", "warning", "degraded", "unknown"
	ImpactLevel    string     `json:"impact_level"` // "NONE", "LOW", "MEDIUM", "HIGH", or ""
	ImpactScore    float64    `json:"impact_score"`
	LastAnalyzedAt *time.Time `json:"last_analyzed_at,omitempty"`
}

// HealthStatusFromImpactLevel maps an impact level string to a traffic-light status.
func HealthStatusFromImpactLevel(level string) string {
	switch level {
	case "NONE", "LOW":
		return "healthy"
	case "MEDIUM":
		return "warning"
	case "HIGH":
		return "degraded"
	default:
		return "unknown"
	}
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
