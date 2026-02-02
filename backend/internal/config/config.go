package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type PrometheusMetric struct {
	Name  string `yaml:"name"`
	Query string `yaml:"query"`
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
	} `yaml:"kubernetes"`
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
		"error_rate":        `avg_over_time(sum(rate(http_requests_total{service=~"{{ .Services }}",status=~"5.."}[1m]))[{{ .Duration }}])`,
		"latency_p95":       `avg_over_time(histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket{service=~"{{ .Services }}"}[1m])))[{{ .Duration }}])`,
		"rps":              `avg_over_time(sum(rate(http_requests_total{service=~"{{ .Services }}"}[1m]))[{{ .Duration }}])`,
		"cpu_saturation":    `avg_over_time(sum(rate(container_cpu_usage_seconds_total{container=~"{{ .Services }}"}[1m]))[{{ .Duration }}])`,
		"memory_saturation": `avg_over_time(sum(container_memory_usage_bytes{container=~"{{ .Services }}"})[{{ .Duration }}])`,
	}
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

	return cfg, nil
}

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
