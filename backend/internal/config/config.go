package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	DatabaseURL string           `mapstructure:"database_url"`
	Port        string           `mapstructure:"port"`
	Prometheus  PrometheusConfig `mapstructure:"prometheus"`
	Kubernetes  KubernetesConfig `mapstructure:"kubernetes"`
	Analysis    AnalysisConfig   `mapstructure:"analysis"`
}

type PrometheusConfig struct {
	URL     string            `mapstructure:"url"`
	Queries map[string]string `mapstructure:"queries"`
}

type KubernetesConfig struct {
	Enabled           bool     `mapstructure:"enabled"`
	KubeConfigPath    string   `mapstructure:"kube_config_path"`
	Namespaces        []string `mapstructure:"namespaces"`
	RequireAnnotation bool     `mapstructure:"require_annotation"`
	AllowedSources    []string `mapstructure:"allowed_sources"`
}

type AnalysisConfig struct {
	BaselineWindow          string `mapstructure:"baseline_window"`
	ImpactWindow            string `mapstructure:"post_execution_impact_window"`
	OrphanCorrelationWindow string `mapstructure:"orphan_correlation_window"`

	BaselineDur          time.Duration `mapstructure:"-"`
	ImpactDur            time.Duration `mapstructure:"-"`
	OrphanCorrelationDur time.Duration `mapstructure:"-"`
}

func Load(configPath string) (*Config, error) {
	v := viper.New()

	// ----- Config file loading -----
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")

		v.AddConfigPath(".")
		v.AddConfigPath("./backend")
		v.AddConfigPath("./example")
		v.AddConfigPath("../example")
	}

	// ----- Set defaults -----
	setDefaults(v)

	// ----- Read YAML if present -----
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// ----- Environment variable support -----
	v.SetEnvPrefix("VALIANT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicit important bindings
	_ = v.BindEnv("database_url")
	_ = v.BindEnv("port")
	_ = v.BindEnv("prometheus.url")
	_ = v.BindEnv("kubernetes.enabled")

	// ----- Unmarshal -----
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// ----- Parse durations -----
	if err := parseDurations(&cfg); err != nil {
		return nil, fmt.Errorf("error parsing durations: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("database_url",
		"postgres://user:password@localhost:5432/valiant?sslmode=disable")
	v.SetDefault("port", "8080")

	v.SetDefault("prometheus.url", "http://localhost:9090")
	v.SetDefault("prometheus.queries", map[string]string{
		"error_rate":        `avg_over_time(sum(rate(http_requests_total{service=~"{{ .Services }}",status=~"5.."}[1m]))[{{ .Duration }}])`,
		"latency_p95":       `avg_over_time(histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket{service=~"{{ .Services }}"}[1m])))[{{ .Duration }}])`,
		"rps":               `avg_over_time(sum(rate(http_requests_total{service=~"{{ .Services }}"}[1m]))[{{ .Duration }}])`,
		"cpu_saturation":    `avg_over_time(sum(rate(container_cpu_usage_seconds_total{container=~"{{ .Services }}"}[1m]))[{{ .Duration }}])`,
		"memory_saturation": `avg_over_time(sum(container_memory_usage_bytes{container=~"{{ .Services }}"})[{{ .Duration }}])`,
	})

	v.SetDefault("kubernetes.enabled", true)
	v.SetDefault("kubernetes.kube_config_path", "")
	v.SetDefault("kubernetes.namespaces", []string{"default"})
	v.SetDefault("kubernetes.require_annotation", true)
	v.SetDefault("kubernetes.allowed_sources", []string{"argocd", "helm", "cicd"})

	v.SetDefault("analysis.baseline_window", "30m")
	v.SetDefault("analysis.post_execution_impact_window", "30m")
	v.SetDefault("analysis.orphan_correlation_window", "1h")
}

func parseDurations(cfg *Config) error {
	var err error

	cfg.Analysis.BaselineDur, err =
		time.ParseDuration(cfg.Analysis.BaselineWindow)
	if err != nil {
		cfg.Analysis.BaselineDur = 30 * time.Minute
	}

	cfg.Analysis.ImpactDur, err =
		time.ParseDuration(cfg.Analysis.ImpactWindow)
	if err != nil {
		cfg.Analysis.ImpactDur = 30 * time.Minute
	}

	cfg.Analysis.OrphanCorrelationDur, err =
		time.ParseDuration(cfg.Analysis.OrphanCorrelationWindow)
	if err != nil {
		cfg.Analysis.OrphanCorrelationDur = 1 * time.Hour
	}

	return nil
}
