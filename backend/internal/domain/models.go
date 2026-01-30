package domain

import "time"

// ChangeEvent is the normalized representation of any collected change.
type ChangeEvent struct {
	ID               string            `json:"id"`
	Source           string            `json:"source"`     // "kubernetes", "git", "ci-cd"
	ChangeType       string            `json:"change_type"` // "deployment_rollout", "configmap_update", "git_tag"
	Timestamp        time.Time         `json:"timestamp"`
	AffectedServices []string          `json:"affected_services"`
	Metadata         map[string]string `json:"metadata"` // e.g., "image_tag", "commit_sha", "author"
	Summary          string            `json:"summary"`  // Human-readable summary
}

// MetricValues holds the calculated values for a specific time window.
type MetricValues struct {
	ErrorRate        float64 `json:"error_rate"`
	LatencyP95       float64 `json:"latency_p95_ms"`
	RPS              float64 `json:"rps"`
	CPUSaturation    float64 `json:"cpu_saturation_percent"`
	MemorySaturation float64 `json:"memory_saturation_percent"`
}

// ImpactAnalysis represents the full analysis of a single change event.
type ImpactAnalysis struct {
	ChangeEvent      ChangeEvent  `json:"change_event"`
	BaselineMetrics  MetricValues `json:"baseline_metrics"`
	ImpactMetrics    MetricValues `json:"impact_metrics"`
	Deltas           MetricValues `json:"deltas"`           // Percentage change for each metric
	ImpactScore      float64      `json:"impact_score"`      // 0.0 to 1.0
	ImpactLevel      string       `json:"impact_level"`      // "NONE", "LOW", "MEDIUM", "HIGH"
	ConfidenceScore  float64      `json:"confidence_score"` // 0.0 to 1.0
}
