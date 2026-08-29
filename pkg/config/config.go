package config

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application including Semgrep and LLM settings.
type Config struct {
	Semgrep SemgrepConfig
	LLM     LLMConfig
}

// LLMConfig holds configuration for LLM providers.
type LLMConfig struct {
	Provider     string
	Model        string
	OpenAIKey    string
	AnthropicKey string
	GeminiKey    string
}

// SemgrepConfig holds configuration for the Semgrep scanner
type SemgrepConfig struct {
	AppToken      string
	Rules         string
	FailOn        string
	Timeout       time.Duration
	SemgrepExists bool
}

// LoadConfig loads variables from .env and system environment,
// checking for the presence of the semgrep binary and populating LLM settings.
func LoadConfig() (*Config, error) {
	// Attempt to load .env file.
	_ = godotenv.Load()

	appToken := os.Getenv("SEMGREP_APP_TOKEN")

	rules := os.Getenv("SEMGREP_RULES")
	if rules == "" {
		rules = "auto"
	}

	failOn := os.Getenv("SEMGREP_FAIL_ON")
	if failOn == "" {
		failOn = "ERROR"
	}

	timeoutStr := strings.TrimSpace(os.Getenv("SEMGREP_TIMEOUT"))
	timeout := 10 * time.Minute
	if timeoutStr != "" {
		if parsed, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = parsed
		} else if secs, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = time.Duration(secs) * time.Second
		}
	}

	// Verify Semgrep is in PATH
	_, err := exec.LookPath("semgrep")
	semgrepExists := err == nil

	// Resolve LLM configuration
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("LLM_PROVIDER")))
	if provider == "" {
		provider = "openai"
	}

	model := strings.TrimSpace(os.Getenv("LLM_MODEL"))
	if model == "" {
		switch provider {
		case "anthropic":
			model = "claude-3-5-sonnet-latest"
		case "gemini":
			model = "gemini-2.5-flash"
		default:
			model = "gpt-4o-mini"
		}
	}

	return &Config{
		Semgrep: SemgrepConfig{
			AppToken:      appToken,
			Rules:         rules,
			FailOn:        failOn,
			Timeout:       timeout,
			SemgrepExists: semgrepExists,
		},
		LLM: LLMConfig{
			Provider:     provider,
			Model:        model,
			OpenAIKey:    os.Getenv("OPENAI_API_KEY"),
			AnthropicKey: os.Getenv("ANTHROPIC_API_KEY"),
			GeminiKey:    os.Getenv("GEMINI_API_KEY"),
		},
	}, nil
}
