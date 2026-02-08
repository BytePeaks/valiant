package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type PrometheusMetric struct {
	Name  string `yaml:"name" json:"name"`
	Query string `yaml:"query" json:"query"`
	Icon  string `yaml:"icon,omitempty" json:"icon,omitempty"`
}

// LinkTemplate defines a template for generating contextual deep links from event metadata.
type LinkTemplate struct {
	Name        string   `yaml:"name"`         // Display name for the link (e.g., "View Commit on GitHub")
	MetadataHas []string `yaml:"metadata_has"` // List of metadata keys that must be present for this template to apply
	URLTemplate string   `yaml:"url_template"` // Go template string for the URL (e.g., "{{ .repository_url }}/commit/{{ .git_commit_sha }}")
}

type PrometheusConfig struct {
	URL               string             `yaml:"url"`
	Queries           map[string]string  `yaml:"queries"`
	AdditionalMetrics []PrometheusMetric `yaml:"additional_metrics"`
}

type AnalysisConfig struct {
	BaselineWindow                   string             `yaml:"baseline_window"` // e.g., "30m"
	ImpactWindow                     string             `yaml:"post_execution_impact_window"`
	BaselineDur                      time.Duration      `yaml:"-"`
	ImpactDur                        time.Duration      `yaml:"-"`
	IntentExecutionCorrelationWindow string             `yaml:"intent_execution_correlation_window"`
	IntentExecutionCorrelationDur    time.Duration      `yaml:"-"`
	WeightsBuiltIn                   map[string]float64 `yaml:"weights_built_in"`
	WeightsCustom                    map[string]float64 `yaml:"weights_custom"`
}

type WorkerConfig struct {
	PollingInterval    string        `yaml:"polling_interval"`
	PollingIntervalDur time.Duration `yaml:"-"`
}

type Config struct {
	DatabaseURL string           `yaml:"database_url"`
	Port        string           `yaml:"port"`
	Prometheus  PrometheusConfig `yaml:"prometheus"`
	Kubernetes  struct {
		Enabled           bool     `yaml:"enabled"`
		KubeConfigPath    string   `yaml:"kube_config_path"`
		Namespaces        []string `yaml:"namespaces"`
		RequireAnnotation bool     `yaml:"require_annotation"`
		AllowedSources    []string `yaml:"allowed_sources"`
		WatchConfigMaps   bool     `yaml:"watch_configmaps"`
		WatchSecrets      bool     `yaml:"watch_secrets"`
	} `yaml:"kubernetes"`
	Retention struct {
		EventTTL           string        `yaml:"event_ttl"`
		EventTTLDur        time.Duration `yaml:"-"`
		CleanupInterval    string        `yaml:"cleanup_interval"`
		CleanupIntervalDur time.Duration `yaml:"-"`
	} `yaml:"retention"`
	Analysis AnalysisConfig `yaml:"analysis"`
	Linking  []LinkTemplate `yaml:"linking"` // deep linking configuration
	Worker   WorkerConfig   `yaml:"worker"`  // New worker configuration
}

func Load(configPath string) (*Config, error) {
	// Default config
	cfg := &Config{
		DatabaseURL: GetEnv("DATABASE_URL", "postgres://user:password@localhost:5432/valiant?sslmode=disable"),
		Port:        GetEnv("PORT", "8080"),
	}
	cfg.Prometheus.URL = GetEnv("PROMETHEUS_URL", "http://localhost:9090")
	cfg.Prometheus.Queries = map[string]string{
		"error_rate":     `avg_over_time(sum(rate(http_requests_total{service=~"{{ .Services }}",status=~"5.."}[1m]))[{{ .Duration }}])`,
		"latency_p95_ms": `avg_over_time(histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket{service=~"{{ .Services }}"}[1m])))[{{ .Duration }}])`,
		"rps":            `avg_over_time(sum(rate(http_requests_total{service=~"{{ .Services }}"}[1m]))[{{ .Duration }}])`,
		"cpu":            `avg_over_time(sum(rate(container_cpu_usage_seconds_total{container=~"{{ .Services }}"}[1m]))[{{ .Duration }}])`,
		"memory":         `avg_over_time(sum(container_memory_usage_bytes{container=~"{{ .Services }}"})[{{ .Duration }}])`,
	}
	cfg.Retention.EventTTL = "90d"
	cfg.Retention.CleanupInterval = "1h"
	cfg.Analysis.BaselineWindow = "30m"
	cfg.Analysis.ImpactWindow = "30m"
	cfg.Analysis.IntentExecutionCorrelationWindow = "1h"
	cfg.Analysis.WeightsBuiltIn = map[string]float64{
		"error_rate":     0.4,
		"latency_p95_ms": 0.3,
		"cpu":            0.1,
		"memory":         0.1,
		"rps":            0.1,
	}
	cfg.Analysis.WeightsCustom = map[string]float64{}
	cfg.Worker.PollingInterval = "5m" // Default worker polling interval

	// Load from YAML if exists, potentially overriding defaults
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, err
			}
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		}
	}

	// Parse durations
	var err error
	cfg.Analysis.BaselineDur, err = time.ParseDuration(cfg.Analysis.BaselineWindow)
	if err != nil {
		cfg.Analysis.BaselineDur = 30 * time.Minute
	}

	cfg.Analysis.ImpactDur, err = time.ParseDuration(cfg.Analysis.ImpactWindow)
	if err != nil {
		cfg.Analysis.ImpactDur = 30 * time.Minute
	}

	cfg.Analysis.IntentExecutionCorrelationDur, err = time.ParseDuration(cfg.Analysis.IntentExecutionCorrelationWindow)
	if err != nil {
		cfg.Analysis.IntentExecutionCorrelationDur = 1 * time.Hour
	}

	cfg.Retention.EventTTLDur, err = parseDuration(cfg.Retention.EventTTL)
	if err != nil {
		cfg.Retention.EventTTLDur = 90 * 24 * time.Hour
	}

	cfg.Retention.CleanupIntervalDur, err = parseDuration(cfg.Retention.CleanupInterval)
	if err != nil {
		cfg.Retention.CleanupIntervalDur = 1 * time.Hour
	}

	// Parse worker polling interval
	cfg.Worker.PollingIntervalDur, err = time.ParseDuration(cfg.Worker.PollingInterval)
	if err != nil {
		cfg.Worker.PollingIntervalDur = 5 * time.Minute // Default to 5 minutes if parsing fails
	}

	return cfg, nil
} 

// parseDuration extends time.ParseDuration with support for a "d" suffix (days).
func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		var days float64
		if _, err := fmt.Sscanf(numStr, "%f", &days); err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		return time.Duration(days * 24 * float64(time.Hour)), nil
	}
	return time.ParseDuration(s)
}

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
