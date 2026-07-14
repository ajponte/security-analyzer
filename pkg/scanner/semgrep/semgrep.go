package semgrep

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"security-analyzer/pkg/config"
)

// SemgrepScanner implements the Scanner interface for Semgrep.
type SemgrepScanner struct {
	cfg *config.SemgrepConfig
}

// NewSemgrepScanner creates a new SemgrepScanner with the given config.
func NewSemgrepScanner(cfg *config.SemgrepConfig) *SemgrepScanner {
	return &SemgrepScanner{cfg: cfg}
}

// Scan runs a Semgrep scan on the target path and returns a ScanReport.
func (s *SemgrepScanner) Scan(ctx context.Context, targetPath string) (*ScanReport, error) {
	if !s.cfg.SemgrepExists {
		return nil, fmt.Errorf("semgrep CLI is not installed or not found in system PATH")
	}

	// Prepare command arguments.
	args := []string{"scan", "--json", fmt.Sprintf("--config=%s", s.cfg.Rules), targetPath}
	cmd := exec.CommandContext(ctx, "semgrep", args...)

	// Configure environment variables (including SEMGREP_APP_TOKEN).
	cmd.Env = os.Environ()
	if s.cfg.AppToken != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("SEMGREP_APP_TOKEN=%s", s.cfg.AppToken))
	}

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	// Semgrep may return a non-zero exit code if findings are found or on configuration issues.
	// We check if the JSON stdout is parsed successfully first.
	if stdoutBuf.Len() > 0 {
		var report ScanReport
		if jsonErr := json.Unmarshal(stdoutBuf.Bytes(), &report); jsonErr == nil {
			return &report, nil
		}
	}

	if err != nil {
		return nil, fmt.Errorf("semgrep execution failed: %w (stderr: %s)", err, stderrBuf.String())
	}

	return nil, fmt.Errorf("semgrep run completed but output was empty or invalid JSON (stderr: %s)", stderrBuf.String())
}
