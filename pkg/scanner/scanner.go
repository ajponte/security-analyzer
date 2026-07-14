package scanner

import (
	"context"

	"security-analyzer/pkg/scanner/semgrep"
)

// Scanner defines the interface for running security scans on a target path.
type Scanner interface {
	Scan(ctx context.Context, targetPath string) (*semgrep.ScanReport, error)
}
