package config

import (
	"os"
	"os/exec"
	"time"

	"github.com/joho/godotenv"
)

// SemgrepConfig holds configuration for the Semgrep scanner
type SemgrepConfig struct {
	AppToken      string
	Rules         string
	FailOn        string
	Timeout       time.Duration
	SemgrepExists bool
}

// LoadConfig loads variables from .env and system environment,
// checking for the presence of the semgrep binary.
func LoadConfig() (*SemgrepConfig, error) {
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

	timeoutStr := os.Getenv("SEMGREP_TIMEOUT")
	timeout := 10 * time.Minute
	if timeoutStr != "" {
		if parsed, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = parsed
		}
	}

	// Verify Semgrep is in PATH
	_, err := exec.LookPath("semgrep")
	semgrepExists := err == nil

	return &SemgrepConfig{
		AppToken:      appToken,
		Rules:         rules,
		FailOn:        failOn,
		Timeout:       timeout,
		SemgrepExists: semgrepExists,
	}, nil
}
