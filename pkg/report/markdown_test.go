package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"security-analyzer/pkg/scanner/semgrep"
)

func TestMarkdownReporter(t *testing.T) {
	tempDir := t.TempDir()
	reportFile := filepath.Join(tempDir, "test-report.md")

	scanReport := &semgrep.ScanReport{
		Version: "1.0.0",
		Results: []semgrep.Result{
			{
				CheckID: "rules.go-sql-injection",
				Path:    "main.go",
				Start:   semgrep.Position{Line: 15, Col: 8},
				Extra: semgrep.Extra{
					Message:  "SQL Injection vulnerability",
					Severity: "ERROR",
					Lines:    "db.Query(fmt.Sprintf(...))",
				},
			},
		},
		Paths: semgrep.Paths{
			Scanned: []string{"main.go"},
		},
	}

	reporter := NewMarkdownReporter(reportFile)
	err := reporter.WriteReport(scanReport)
	if err != nil {
		t.Fatalf("WriteReport failed: %v", err)
	}

	content, err := os.ReadFile(reportFile)
	if err != nil {
		t.Fatalf("failed to read written report file: %v", err)
	}

	contentStr := string(content)

	if !strings.Contains(contentStr, "# Semgrep Security Scan Report") {
		t.Errorf("missing main title")
	}

	if !strings.Contains(contentStr, "## Severity: ERROR (1)") {
		t.Errorf("missing severity block")
	}

	if !strings.Contains(contentStr, "rules.go-sql-injection") {
		t.Errorf("missing CheckID")
	}

	if !strings.Contains(contentStr, "main.go:15:8") {
		t.Errorf("missing file location info")
	}

	if !strings.Contains(contentStr, "SQL Injection vulnerability") {
		t.Errorf("missing message info")
	}

	if !strings.Contains(contentStr, "db.Query(fmt.Sprintf(...))") {
		t.Errorf("missing code context")
	}
}
