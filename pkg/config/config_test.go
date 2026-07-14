package config

import (
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	t.Setenv("SEMGREP_APP_TOKEN", "mock-token")
	t.Setenv("SEMGREP_RULES", "p/golang")
	t.Setenv("SEMGREP_FAIL_ON", "WARNING")
	t.Setenv("SEMGREP_TIMEOUT", "5m")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.AppToken != "mock-token" {
		t.Errorf("expected AppToken to be 'mock-token', got %q", cfg.AppToken)
	}

	if cfg.Rules != "p/golang" {
		t.Errorf("expected Rules to be 'p/golang', got %q", cfg.Rules)
	}

	if cfg.FailOn != "WARNING" {
		t.Errorf("expected FailOn to be 'WARNING', got %q", cfg.FailOn)
	}

	if cfg.Timeout != 5*time.Minute {
		t.Errorf("expected Timeout to be 5m, got %v", cfg.Timeout)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("SEMGREP_APP_TOKEN", "")
	t.Setenv("SEMGREP_RULES", "")
	t.Setenv("SEMGREP_FAIL_ON", "")
	t.Setenv("SEMGREP_TIMEOUT", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.Rules != "auto" {
		t.Errorf("expected default Rules to be 'auto', got %q", cfg.Rules)
	}

	if cfg.FailOn != "ERROR" {
		t.Errorf("expected default FailOn to be 'ERROR', got %q", cfg.FailOn)
	}

	if cfg.Timeout != 10*time.Minute {
		t.Errorf("expected default Timeout to be 10m, got %v", cfg.Timeout)
	}
}
