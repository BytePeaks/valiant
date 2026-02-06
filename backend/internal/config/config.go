package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type PrometheusMetric struct {
	Name  string `yaml:"name"`
	Query string `yaml:"query"`
	Icon  string `yaml:"icon,omitempty"`
}

type Config struct {
	DatabaseURL string `yaml:"database_url"`
	Port        string `yaml:"port"`

	Prometheus struct {
		URL               string              `yaml:"url"`
		Queries           map[string]string   `yaml:"queries"`
		AdditionalMetrics []PrometheusMetric  `yaml:"additional_metrics"`
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
		BaselineWindow                   string        `yaml:"baseline_window"`
		ImpactWindow                     string        `yaml:"post_execution_impact_window"`
		IntentExecutionCorrelationWindow string        `yaml:"intent_execution_correlation_window"`

		BaselineDur                 time.Duration `yaml:"-"`
		ImpactDur                   time.Duration `yaml:"-"`
		IntentExecutionCorrelationDur time.Duration `yaml:"-"`
	} `yaml:"analysis"`
}

func Load(configPath string) (*Config, error) {
	v := viper.New()

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

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// ----- ENV OVERRIDE (YOUR CONTRIBUTION) -----
	v.SetEnvPrefix("VALIANT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	_ = v.BindEnv("database_url")
	_ = v.BindEnv("port")
	_ = v.BindEnv("prometheus.url")
	_ = v.BindEnv("kubernetes.enabled")
	// --------------------------------------------

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// defaults for metrics if not provided
	if cfg.Prometheus.Queries == nil {
		cfg.Prometheus.Queries = map[string]string{
			"error_rate":   `avg_over_time(sum(rate(http_requests_total{service=~"{{ .Services }}",status=~"5.."}[1m]))[{{ .Duration }}])`,
			"latency_p95":  `avg_over_time(histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket{service=~"{{ .Services }}"}[1m])))[{{ .Duration }}])`,
			"rps":          `avg_over_time(sum(rate(http_requests_total{service=~"{{ .Services }}"}[1m]))[{{ .Duration }}])`,
		}
	}

	if err := parseDurations(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func parseDurations(cfg *Config) error {
	var err error

	cfg.Analysis.BaselineDur, err = time.ParseDuration(cfg.Analysis.BaselineWindow)
	if err != nil {
		cfg.Analysis.BaselineDur = 30 * time.Minute
	}

	cfg.Analysis.ImpactDur, err = time.ParseDuration(cfg.Analysis.ImpactWindow)
	if err != nil {
		cfg.Analysis.ImpactDur = 30 * time.Minute
	}

	cfg.Analysis.IntentExecutionCorrelationDur, err =
		time.ParseDuration(cfg.Analysis.IntentExecutionCorrelationWindow)
	if err != nil {
		cfg.Analysis.IntentExecutionCorrelationDur = 1 * time.Hour
	}

	cfg.Retention.EventTTLDur, err = parseDuration(cfg.Retention.EventTTL)
	if err != nil {
		cfg.Retention.EventTTLDur = 90 * 24 * time.Hour
	}

	cfg.Retention.CleanupIntervalDur, err =
		parseDuration(cfg.Retention.CleanupInterval)
	if err != nil {
		cfg.Retention.CleanupIntervalDur = 1 * time.Hour
	}

	return nil
}

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
