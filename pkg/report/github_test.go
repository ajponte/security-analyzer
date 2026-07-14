package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"security-analyzer/pkg/scanner/semgrep"
)

func TestGitHubReporter(t *testing.T) {
	tempDir := t.TempDir()
	summaryFile := filepath.Join(tempDir, "github-summary.md")

	t.Setenv("GITHUB_STEP_SUMMARY", summaryFile)

	scanReport := &semgrep.ScanReport{
		Version: "1.0.0",
		Results: []semgrep.Result{
			{
				CheckID: "rules.go-hardcoded-secret",
				Path:    "config.go",
				Start:   semgrep.Position{Line: 10, Col: 12},
				Extra: semgrep.Extra{
					Message:  "Hardcoded secret key",
					Severity: "ERROR",
				},
			},
			{
				CheckID: "rules.go-weak-crypto",
				Path:    "crypto.go",
				Start:   semgrep.Position{Line: 20, Col: 5},
				Extra: semgrep.Extra{
					Message:  "Weak cryptographic algorithm",
					Severity: "WARNING",
				},
			},
		},
		Paths: semgrep.Paths{
			Scanned: []string{"config.go", "crypto.go"},
		},
	}

	reporter := NewGitHubReporter()
	err := reporter.WriteReport(scanReport)
	if err != nil {
		t.Fatalf("WriteReport failed: %v", err)
	}

	content, err := os.ReadFile(summaryFile)
	if err != nil {
		t.Fatalf("failed to read written summary file: %v", err)
	}

	contentStr := string(content)

	if !strings.Contains(contentStr, "### 🛡️ Semgrep Security Scan Summary") {
		t.Errorf("missing summary title")
	}

	if !strings.Contains(contentStr, "| 🔴 **Error** | 1 |") {
		t.Errorf("missing error count row")
	}

	if !strings.Contains(contentStr, "| ⚠️ **Warning** | 1 |") {
		t.Errorf("missing warning count row")
	}

	if !strings.Contains(contentStr, "🔴 | `config.go` | 10 | `rules.go-hardcoded-secret` | Hardcoded secret key |") {
		t.Errorf("missing detailed finding error row")
	}

	if !strings.Contains(contentStr, "⚠️ | `crypto.go` | 20 | `rules.go-weak-crypto` | Weak cryptographic algorithm |") {
		t.Errorf("missing detailed finding warning row")
	}
}

func TestGitHubReporterNotConfigured(t *testing.T) {
	t.Setenv("GITHUB_STEP_SUMMARY", "")

	scanReport := &semgrep.ScanReport{
		Version: "1.0.0",
		Results: []semgrep.Result{},
	}

	reporter := NewGitHubReporter()
	err := reporter.WriteReport(scanReport)
	if err != nil {
		t.Fatalf("WriteReport should do nothing and not fail when GITHUB_STEP_SUMMARY is empty, got error: %v", err)
	}
}
