package semgrep

import (
	"encoding/json"
	"testing"
)

func TestParseScanReport(t *testing.T) {
	mockJSON := `{
		"version": "1.0.0",
		"results": [
			{
				"check_id": "rules.go-sql-injection",
				"path": "main.go",
				"start": {
					"line": 15,
					"col": 8,
					"offset": 250
				},
				"end": {
					"line": 15,
					"col": 40,
					"offset": 282
				},
				"extra": {
					"message": "SQL Injection vulnerability",
					"severity": "ERROR",
					"lines": "db.Query(fmt.Sprintf(\"SELECT * FROM users WHERE id = %s\", id))"
				}
			}
		],
		"errors": [
			{
				"code": 4001,
				"message": "failed to parse config",
				"type": "ConfigError",
				"path": "config.yaml"
			}
		],
		"paths": {
			"scanned": ["main.go", "utils.go"]
		}
	}`

	var report ScanReport
	err := json.Unmarshal([]byte(mockJSON), &report)
	if err != nil {
		t.Fatalf("failed to unmarshal mock JSON: %v", err)
	}

	if report.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", report.Version)
	}

	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}

	res := report.Results[0]
	if res.CheckID != "rules.go-sql-injection" {
		t.Errorf("expected CheckID to be 'rules.go-sql-injection', got %q", res.CheckID)
	}

	if res.Path != "main.go" {
		t.Errorf("expected Path to be 'main.go', got %q", res.Path)
	}

	if res.Start.Line != 15 || res.Start.Col != 8 {
		t.Errorf("expected start position (15, 8), got (%d, %d)", res.Start.Line, res.Start.Col)
	}

	if res.Extra.Severity != "ERROR" {
		t.Errorf("expected severity 'ERROR', got %q", res.Extra.Severity)
	}

	if len(report.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(report.Errors))
	}

	errObj := report.Errors[0]
	if errObj.Code != 4001 || errObj.Type != "ConfigError" {
		t.Errorf("expected error code 4001 and type 'ConfigError', got %d, %s", errObj.Code, errObj.Type)
	}

	if len(report.Paths.Scanned) != 2 {
		t.Errorf("expected 2 scanned paths, got %d", len(report.Paths.Scanned))
	}
}
