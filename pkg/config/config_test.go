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
	t.Setenv("LLM_PROVIDER", "anthropic")
	t.Setenv("LLM_MODEL", "claude-custom")
	t.Setenv("OPENAI_API_KEY", "o-key")
	t.Setenv("ANTHROPIC_API_KEY", "a-key")
	t.Setenv("GEMINI_API_KEY", "g-key")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.Semgrep.AppToken != "mock-token" {
		t.Errorf("expected AppToken to be 'mock-token', got %q", cfg.Semgrep.AppToken)
	}

	if cfg.Semgrep.Rules != "p/golang" {
		t.Errorf("expected Rules to be 'p/golang', got %q", cfg.Semgrep.Rules)
	}

	if cfg.Semgrep.FailOn != "WARNING" {
		t.Errorf("expected FailOn to be 'WARNING', got %q", cfg.Semgrep.FailOn)
	}

	if cfg.Semgrep.Timeout != 5*time.Minute {
		t.Errorf("expected Timeout to be 5m, got %v", cfg.Semgrep.Timeout)
	}

	if cfg.LLM.Provider != "anthropic" {
		t.Errorf("expected Provider to be 'anthropic', got %q", cfg.LLM.Provider)
	}

	if cfg.LLM.Model != "claude-custom" {
		t.Errorf("expected Model to be 'claude-custom', got %q", cfg.LLM.Model)
	}

	if cfg.LLM.OpenAIKey != "o-key" || cfg.LLM.AnthropicKey != "a-key" || cfg.LLM.GeminiKey != "g-key" {
		t.Errorf("keys did not match expected values: %+v", cfg.LLM)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("SEMGREP_APP_TOKEN", "")
	t.Setenv("SEMGREP_RULES", "")
	t.Setenv("SEMGREP_FAIL_ON", "")
	t.Setenv("SEMGREP_TIMEOUT", "")
	t.Setenv("LLM_PROVIDER", "")
	t.Setenv("LLM_MODEL", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.Semgrep.Rules != "auto" {
		t.Errorf("expected default Rules to be 'auto', got %q", cfg.Semgrep.Rules)
	}

	if cfg.Semgrep.FailOn != "ERROR" {
		t.Errorf("expected default FailOn to be 'ERROR', got %q", cfg.Semgrep.FailOn)
	}

	if cfg.Semgrep.Timeout != 10*time.Minute {
		t.Errorf("expected default Timeout to be 10m, got %v", cfg.Semgrep.Timeout)
	}

	if cfg.LLM.Provider != "openai" {
		t.Errorf("expected default LLM provider 'openai', got %q", cfg.LLM.Provider)
	}

	if cfg.LLM.Model != "gpt-4o-mini" {
		t.Errorf("expected default LLM model 'gpt-4o-mini', got %q", cfg.LLM.Model)
	}
}

func TestLoadConfig_CaseInsensitiveProvider(t *testing.T) {
	t.Run("Mixed case Anthropic default model", func(t *testing.T) {
		t.Setenv("LLM_PROVIDER", " Anthropic ")
		t.Setenv("LLM_MODEL", "")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.LLM.Provider != "anthropic" {
			t.Errorf("expected provider 'anthropic', got %q", cfg.LLM.Provider)
		}
		if cfg.LLM.Model != "claude-3-5-sonnet-latest" {
			t.Errorf("expected default model 'claude-3-5-sonnet-latest', got %q", cfg.LLM.Model)
		}
	})

	t.Run("Uppercase Gemini default model", func(t *testing.T) {
		t.Setenv("LLM_PROVIDER", "GEMINI")
		t.Setenv("LLM_MODEL", "")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.LLM.Provider != "gemini" {
			t.Errorf("expected provider 'gemini', got %q", cfg.LLM.Provider)
		}
		if cfg.LLM.Model != "gemini-2.5-flash" {
			t.Errorf("expected default model 'gemini-2.5-flash', got %q", cfg.LLM.Model)
		}
	})
}

func TestLoadConfig_TimeoutFormats(t *testing.T) {
	t.Run("Duration string with unit (e.g. 5m)", func(t *testing.T) {
		t.Setenv("SEMGREP_TIMEOUT", "5m")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Semgrep.Timeout != 5*time.Minute {
			t.Errorf("expected 5m, got %v", cfg.Semgrep.Timeout)
		}
	})

	t.Run("Integer seconds (e.g. 300)", func(t *testing.T) {
		t.Setenv("SEMGREP_TIMEOUT", "300")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Semgrep.Timeout != 300*time.Second {
			t.Errorf("expected 300s (5m), got %v", cfg.Semgrep.Timeout)
		}
	})

	t.Run("Integer seconds with whitespace (e.g. ' 60 ')", func(t *testing.T) {
		t.Setenv("SEMGREP_TIMEOUT", " 60 ")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Semgrep.Timeout != 60*time.Second {
			t.Errorf("expected 60s, got %v", cfg.Semgrep.Timeout)
		}
	})
}
