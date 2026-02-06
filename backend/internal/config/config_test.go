package config

import (
	"os"
	"testing"
)

func TestLoad_WithEnvVars(t *testing.T) {
	os.Setenv("VALIANT_DATABASE_URL", "postgres://testuser:testpass@testdb:5432/testdb")
	os.Setenv("VALIANT_PORT", "9000")
	os.Setenv("VALIANT_PROMETHEUS_URL", "http://test-prometheus:9090")
	os.Setenv("VALIANT_KUBERNETES_ENABLED", "false")
	os.Setenv("VALIANT_ANALYSIS_BASELINE_WINDOW", "1h")

	defer func() {
		os.Unsetenv("VALIANT_DATABASE_URL")
		os.Unsetenv("VALIANT_PORT")
		os.Unsetenv("VALIANT_PROMETHEUS_URL")
		os.Unsetenv("VALIANT_KUBERNETES_ENABLED")
		os.Unsetenv("VALIANT_ANALYSIS_BASELINE_WINDOW")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DatabaseURL != "postgres://testuser:testpass@testdb:5432/testdb" {
		t.Errorf("Expected DATABASE_URL from env var, got: %s", cfg.DatabaseURL)
	}

	if cfg.Port != "9000" {
		t.Errorf("Expected port 9000, got: %s", cfg.Port)
	}

	if cfg.Prometheus.URL != "http://test-prometheus:9090" {
		t.Errorf("Expected Prometheus URL from env var, got: %s", cfg.Prometheus.URL)
	}

	if cfg.Kubernetes.Enabled != false {
		t.Errorf("Expected kubernetes.enabled to be false from env var")
	}

	if cfg.Analysis.BaselineWindow != "1h" {
		t.Errorf("Expected baseline_window 1h from env var, got: %s", cfg.Analysis.BaselineWindow)
	}
}

func TestLoad_LegacyEnvVars(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://legacy:pass@db:5432/legacy")
	os.Setenv("PORT", "7000")
	os.Setenv("PROMETHEUS_URL", "http://legacy-prom:9090")

	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("PORT")
		os.Unsetenv("PROMETHEUS_URL")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DatabaseURL != "postgres://legacy:pass@db:5432/legacy" {
		t.Errorf("Expected legacy DATABASE_URL, got: %s", cfg.DatabaseURL)
	}

	if cfg.Port != "7000" {
		t.Errorf("Expected port 7000, got: %s", cfg.Port)
	}
}

func TestLoad_WithDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("Expected default port 8080, got: %s", cfg.Port)
	}

	if cfg.Kubernetes.Enabled != true {
		t.Errorf("Expected kubernetes.enabled to be true by default")
	}

	if cfg.Analysis.BaselineWindow != "30m" {
		t.Errorf("Expected default baseline_window 30m, got: %s", cfg.Analysis.BaselineWindow)
	}

	if cfg.Analysis.BaselineDur.Minutes() != 30 {
		t.Errorf("Expected baseline duration 30 minutes, got: %v", cfg.Analysis.BaselineDur)
	}
}

func TestLoad_DurationParsing(t *testing.T) {
	os.Setenv("VALIANT_ANALYSIS_BASELINE_WINDOW", "2h")
	os.Setenv("VALIANT_ANALYSIS_POST_EXECUTION_IMPACT_WINDOW", "45m")
	defer func() {
		os.Unsetenv("VALIANT_ANALYSIS_BASELINE_WINDOW")
		os.Unsetenv("VALIANT_ANALYSIS_POST_EXECUTION_IMPACT_WINDOW")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Analysis.BaselineDur.Hours() != 2 {
		t.Errorf("Expected 2 hours, got: %v", cfg.Analysis.BaselineDur)
	}

	if cfg.Analysis.ImpactDur.Minutes() != 45 {
		t.Errorf("Expected 45 minutes, got: %v", cfg.Analysis.ImpactDur)
	}
}
