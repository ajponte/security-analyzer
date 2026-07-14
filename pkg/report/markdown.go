package report

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"security-analyzer/pkg/scanner/semgrep"
)

// MarkdownReporter generates a Markdown report file of Semgrep findings.
type MarkdownReporter struct {
	outputPath string
}

// NewMarkdownReporter creates a new MarkdownReporter writing to the specified file.
func NewMarkdownReporter(outputPath string) *MarkdownReporter {
	if outputPath == "" {
		outputPath = "report.md"
	}
	return &MarkdownReporter{outputPath: outputPath}
}

// WriteReport formats and writes the Semgrep scan findings into the markdown file.
func (r *MarkdownReporter) WriteReport(scanReport *semgrep.ScanReport) error {
	dir := filepath.Dir(r.outputPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory for markdown report: %w", err)
		}
	}

	file, err := os.Create(r.outputPath)
	if err != nil {
		return fmt.Errorf("failed to create markdown report file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var buf bytes.Buffer

	fmt.Fprintln(&buf, "# Semgrep Security Scan Report")
	fmt.Fprintln(&buf)
	fmt.Fprintf(&buf, "- **Total Scanned Files**: %d\n", len(scanReport.Paths.Scanned))
	fmt.Fprintf(&buf, "- **Total Findings**: %d\n", len(scanReport.Results))
	fmt.Fprintf(&buf, "- **Errors**: %d\n", len(scanReport.Errors))
	fmt.Fprintln(&buf)

	if len(scanReport.Results) == 0 {
		fmt.Fprintln(&buf, "## 🎉 No findings detected!")
	} else {
		// Group findings by severity.
		bySeverity := make(map[string][]semgrep.Result)
		for _, res := range scanReport.Results {
			sev := res.Extra.Severity
			if sev == "" {
				sev = "UNKNOWN"
			}
			bySeverity[sev] = append(bySeverity[sev], res)
		}

		// Define priority order for reporting.
		severities := []string{"ERROR", "WARNING", "INFO"}
		for sev := range bySeverity {
			found := false
			for _, s := range severities {
				if s == sev {
					found = true
					break
				}
			}
			if !found {
				severities = append(severities, sev)
			}
		}

		for _, sev := range severities {
			findings, ok := bySeverity[sev]
			if !ok || len(findings) == 0 {
				continue
			}

			fmt.Fprintf(&buf, "## Severity: %s (%d)\n\n", sev, len(findings))
			for i, f := range findings {
				fmt.Fprintf(&buf, "### %d. %s\n", i+1, f.CheckID)
				fmt.Fprintf(&buf, "- **File**: `%s:%d:%d`\n", f.Path, f.Start.Line, f.Start.Col)
				fmt.Fprintf(&buf, "- **Message**: %s\n", f.Extra.Message)
				if f.Extra.Lines != "" {
					fmt.Fprintln(&buf, "- **Code Context**:")
					fmt.Fprintln(&buf, "  ```")
					fmt.Fprintf(&buf, "  %s\n", f.Extra.Lines)
					fmt.Fprintln(&buf, "  ```")
				}
				fmt.Fprintln(&buf)
			}
		}
	}

	if len(scanReport.Errors) > 0 {
		fmt.Fprintln(&buf, "## Scan Errors")
		for _, e := range scanReport.Errors {
			fmt.Fprintf(&buf, "- **Path**: `%s` (Code: %d): %s (%s)\n", e.Path, e.Code, e.Message, e.Type)
		}
	}

	_, err = file.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write markdown report: %w", err)
	}

	return nil
}
