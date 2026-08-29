package semgrep

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

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

	// Configure execution context timeout.
	if s.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.cfg.Timeout)
		defer cancel()
	}

	// Prepare command arguments with --quiet to ensure clean JSON stdout without terminal banner formatting.
	args := []string{"scan", "--json", "--quiet", fmt.Sprintf("--config=%s", s.cfg.Rules), targetPath}
	cmd := exec.CommandContext(ctx, "semgrep", args...)

	// Configure environment variables (including SEMGREP_APP_TOKEN and integer SEMGREP_TIMEOUT).
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "SEMGREP_TIMEOUT=") {
			env = append(env, e)
		}
	}
	if s.cfg.AppToken != "" {
		env = append(env, fmt.Sprintf("SEMGREP_APP_TOKEN=%s", s.cfg.AppToken))
	}
	if s.cfg.Timeout > 0 {
		secs := int(s.cfg.Timeout.Seconds())
		env = append(env, fmt.Sprintf("SEMGREP_TIMEOUT=%d", secs))
	}
	cmd.Env = env

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	// Semgrep may return a non-zero exit code if findings are found or on configuration issues.
	// We check if the JSON stdout is parsed successfully first.
	if stdoutBuf.Len() > 0 {
		report, parseErr := parseScanReport(stdoutBuf.Bytes())
		if parseErr == nil {
			if report.ScanID == "" {
				report.ScanID = generateScanID()
			}
			return report, nil
		}
	}

	if err != nil {
		return nil, fmt.Errorf("semgrep execution failed: %w (stderr: %s)", err, stderrBuf.String())
	}

	return nil, fmt.Errorf("semgrep run completed but output was empty or invalid JSON (stderr: %s)", stderrBuf.String())
}

func parseScanReport(data []byte) (*ScanReport, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty output")
	}

	var report ScanReport
	if err := json.Unmarshal(data, &report); err == nil {
		return &report, nil
	}

	// Slicing fallback: locate JSON object boundaries if surrounding terminal output exists
	start := bytes.IndexByte(data, '{')
	end := bytes.LastIndexByte(data, '}')
	if start != -1 && end != -1 && end > start {
		if err := json.Unmarshal(data[start:end+1], &report); err == nil {
			return &report, nil
		}
	}

	return nil, fmt.Errorf("invalid JSON payload")
}

func generateScanID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("scan-%s-%s", time.Now().Format("20060102-150405"), hex.EncodeToString(b))
}
