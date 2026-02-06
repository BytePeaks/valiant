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

type Config struct {
	DatabaseURL string `yaml:"database_url"`
	Port        string `yaml:"port"`
	Prometheus  struct {
		URL              string            `yaml:"url"`
		Queries          map[string]string `yaml:"queries"`
		AdditionalMetrics []PrometheusMetric `yaml:"additional_metrics"`
	} `yaml:"prometheus"`
	Kubernetes struct {
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
	Analysis struct {
		BaselineWindow string        `yaml:"baseline_window"` // e.g., "30m"
		ImpactWindow   string        `yaml:"post_execution_impact_window"`
		BaselineDur    time.Duration `yaml:"-"`
		ImpactDur             time.Duration `yaml:"-"`
		IntentExecutionCorrelationWindow string        `yaml:"intent_execution_correlation_window"`
		IntentExecutionCorrelationDur  time.Duration `yaml:"-"`
	} `yaml:"analysis"`
}

func Load(configPath string) (*Config, error) {
	// Default config
	cfg := &Config{
		DatabaseURL: GetEnv("DATABASE_URL", "postgres://user:password@localhost:5432/valiant?sslmode=disable"),
		Port:        GetEnv("PORT", "8080"),
	}
	cfg.Prometheus.URL = GetEnv("PROMETHEUS_URL", "http://localhost:9090")
	cfg.Prometheus.Queries = map[string]string{
		"error_rate":                  `avg_over_time(sum(rate(http_requests_total{service=~"{{ .Services }}",status=~"5.."}[1m]))[{{ .Duration }}])`,
		"latency_p95_ms":              `avg_over_time(histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket{service=~"{{ .Services }}"}[1m])))[{{ .Duration }}])`,
		"rps":                         `avg_over_time(sum(rate(http_requests_total{service=~"{{ .Services }}"}[1m]))[{{ .Duration }}])`,
		"cpu":      `avg_over_time(sum(rate(container_cpu_usage_seconds_total{container=~"{{ .Services }}"}[1m]))[{{ .Duration }}])`,
		"memory":   `avg_over_time(sum(container_memory_usage_bytes{container=~"{{ .Services }}"})[{{ .Duration }}])`,
	}
	cfg.Retention.EventTTL = "90d"
	cfg.Retention.CleanupInterval = "1h"
	cfg.Analysis.BaselineWindow = "30m"
	cfg.Analysis.ImpactWindow = "30m"
	cfg.Analysis.IntentExecutionCorrelationWindow = "1h"

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
