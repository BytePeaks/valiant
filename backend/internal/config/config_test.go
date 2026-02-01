package config_test

import (
	"os"
	"testing"
	"time"
	"valiant/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("expected port 8080, got %s", cfg.Port)
	}
	if cfg.Analysis.BaselineDur != 30*time.Minute {
		t.Errorf("expected baseline 30m, got %v", cfg.Analysis.BaselineDur)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	os.Setenv("PORT", "9999")
	defer os.Unsetenv("PORT")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Port != "9999" {
		t.Errorf("expected port 9999 from env, got %s", cfg.Port)
	}
}

func TestLoad_YAML(t *testing.T) {
	yamlContent := `
port: "7070"
analysis:
  baseline_window: "15m"
  post_execution_impact_window: "1h"
  intent_execution_correlation_window: "2h"
`
	tmpFile := "test_config.yaml"
	err := os.WriteFile(tmpFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile)

	cfg, err := config.Load(tmpFile)
	if err != nil {
		t.Fatalf("failed to load YAML config: %v", err)
	}

	if cfg.Port != "7070" {
		t.Errorf("expected port 7070, got %s", cfg.Port)
	}
	if cfg.Analysis.BaselineDur != 15*time.Minute {
		t.Errorf("expected baseline 15m, got %v", cfg.Analysis.BaselineDur)
	}
	if cfg.Analysis.ImpactDur != 1*time.Hour {
		t.Errorf("expected impact 1h, got %v", cfg.Analysis.ImpactDur)
	}
	if cfg.Analysis.IntentExecutionCorrelationDur != 2*time.Hour {
		t.Errorf("expected orphan correlation 2h, got %v", cfg.Analysis.IntentExecutionCorrelationDur)
	}
}

