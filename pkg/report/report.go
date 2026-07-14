package report

import (
	"security-analyzer/pkg/scanner/semgrep"
)

// Reporter defines the interface for exporting scan reports.
type Reporter interface {
	WriteReport(report *semgrep.ScanReport) error
}
