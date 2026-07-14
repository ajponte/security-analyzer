package report

import (
	"bytes"
	"fmt"
	"os"

	"security-analyzer/pkg/scanner/semgrep"
)

// GitHubReporter formats and appends the scan summary to the GitHub Action Step Summary.
type GitHubReporter struct{}

// NewGitHubReporter creates a new GitHubReporter.
func NewGitHubReporter() *GitHubReporter {
	return &GitHubReporter{}
}

// WriteReport appends the formatted summary to the file path defined in GITHUB_STEP_SUMMARY.
func (r *GitHubReporter) WriteReport(scanReport *semgrep.ScanReport) error {
	summaryPath := os.Getenv("GITHUB_STEP_SUMMARY")
	if summaryPath == "" {
		// Do nothing if GITHUB_STEP_SUMMARY is not configured.
		return nil
	}

	file, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open GITHUB_STEP_SUMMARY file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var buf bytes.Buffer

	errCount := 0
	warnCount := 0
	infoCount := 0
	otherCount := 0

	for _, res := range scanReport.Results {
		switch res.Extra.Severity {
		case "ERROR":
			errCount++
		case "WARNING":
			warnCount++
		case "INFO":
			infoCount++
		default:
			otherCount++
		}
	}

	fmt.Fprintln(&buf, "### 🛡️ Semgrep Security Scan Summary")
	fmt.Fprintln(&buf)
	fmt.Fprintln(&buf, "| Severity | Count |")
	fmt.Fprintln(&buf, "| --- | --- |")
	fmt.Fprintf(&buf, "| 🔴 **Error** | %d |\n", errCount)
	fmt.Fprintf(&buf, "| ⚠️ **Warning** | %d |\n", warnCount)
	fmt.Fprintf(&buf, "| ℹ️ **Info** | %d |\n", infoCount)
	if otherCount > 0 {
		fmt.Fprintf(&buf, "| ❔ **Other** | %d |\n", otherCount)
	}
	fmt.Fprintln(&buf)

	if len(scanReport.Results) > 0 {
		fmt.Fprintln(&buf, "#### Detailed Findings")
		fmt.Fprintln(&buf)
		fmt.Fprintln(&buf, "| Icon | File | Line | Rule ID | Message |")
		fmt.Fprintln(&buf, "| :---: | --- | :---: | --- | --- |")

		for _, res := range scanReport.Results {
			icon := "ℹ️"
			switch res.Extra.Severity {
			case "ERROR":
				icon = "🔴"
			case "WARNING":
				icon = "⚠️"
			}
			fmt.Fprintf(&buf, "| %s | `%s` | %d | `%s` | %s |\n",
				icon, res.Path, res.Start.Line, res.CheckID, res.Extra.Message)
		}
		fmt.Fprintln(&buf)
	} else {
		fmt.Fprintln(&buf, "🎉 **No security findings detected!**")
		fmt.Fprintln(&buf)
	}

	if len(scanReport.Errors) > 0 {
		fmt.Fprintln(&buf, "#### ⚠️ Scan Errors")
		fmt.Fprintln(&buf)
		fmt.Fprintln(&buf, "| File | Code | Message |")
		fmt.Fprintln(&buf, "| --- | --- | --- |")
		for _, e := range scanReport.Errors {
			fmt.Fprintf(&buf, "| `%s` | %d | %s |\n", e.Path, e.Code, e.Message)
		}
		fmt.Fprintln(&buf)
	}

	_, err = file.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write to GITHUB_STEP_SUMMARY: %w", err)
	}

	return nil
}
