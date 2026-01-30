package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DatabaseURL string `yaml:"database_url"`
	Port        string `yaml:"port"`
	Prometheus  struct {
		URL string `yaml:"url"`
	} `yaml:"prometheus"`
	Analysis struct {
		BaselineWindow string        `yaml:"baseline_window"` // e.g., "30m"
		ImpactWindow   string        `yaml:"impact_window"`   // e.g., "30m"
		BaselineDur    time.Duration `yaml:"-"`
		ImpactDur      time.Duration `yaml:"-"`
	} `yaml:"analysis"`
}

func Load(configPath string) (*Config, error) {
	// Default config
	cfg := &Config{
		DatabaseURL: GetEnv("DATABASE_URL", "postgres://user:password@localhost:5432/valiant?sslmode=disable"),
		Port:        GetEnv("PORT", "8080"),
	}
	cfg.Prometheus.URL = GetEnv("PROMETHEUS_URL", "http://localhost:9090")
	cfg.Analysis.BaselineWindow = "30m"
	cfg.Analysis.ImpactWindow = "30m"

	// Load from YAML if exists
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

	return cfg, nil
}

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
