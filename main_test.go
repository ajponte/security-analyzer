package main

import (
	"testing"

	"security-analyzer/pkg/scanner/semgrep"
)

func TestParseCLIArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantMode runMode
		wantPath string
	}{
		{
			name:     "no args defaults to scan on current dir",
			args:     []string{"security-analyzer"},
			wantMode: modeScan,
			wantPath: ".",
		},
		{
			name:     "direct path argument",
			args:     []string{"security-analyzer", "./cmd"},
			wantMode: modeScan,
			wantPath: "./cmd",
		},
		{
			name:     "scan subcommand with path",
			args:     []string{"security-analyzer", "scan", "./pkg"},
			wantMode: modeScan,
			wantPath: "./pkg",
		},
		{
			name:     "scan subcommand without path",
			args:     []string{"security-analyzer", "scan"},
			wantMode: modeScan,
			wantPath: ".",
		},
		{
			name:     "mcp subcommand",
			args:     []string{"security-analyzer", "mcp"},
			wantMode: modeMCP,
			wantPath: ".",
		},
		{
			name:     "mcp subcommand with path",
			args:     []string{"security-analyzer", "mcp", "/target/repo"},
			wantMode: modeMCP,
			wantPath: "/target/repo",
		},
		{
			name:     "analyze subcommand with path",
			args:     []string{"security-analyzer", "analyze", "./internal"},
			wantMode: modeAnalyze,
			wantPath: "./internal",
		},
		{
			name:     "analyze subcommand without path",
			args:     []string{"security-analyzer", "analyze"},
			wantMode: modeAnalyze,
			wantPath: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := parseCLIArgs(tt.args)
			if opts.mode != tt.wantMode {
				t.Errorf("expected mode %q, got %q", tt.wantMode, opts.mode)
			}
			if opts.scanPath != tt.wantPath {
				t.Errorf("expected scanPath %q, got %q", tt.wantPath, opts.scanPath)
			}
		})
	}
}

func TestShouldFailBuild(t *testing.T) {
	errorFinding := semgrep.Result{
		Extra: semgrep.Extra{Severity: "ERROR"},
	}
	warningFinding := semgrep.Result{
		Extra: semgrep.Extra{Severity: "WARNING"},
	}
	infoFinding := semgrep.Result{
		Extra: semgrep.Extra{Severity: "INFO"},
	}

	tests := []struct {
		name     string
		failOn   string
		results  []semgrep.Result
		wantFail bool
	}{
		{
			name:     "ERROR policy with no findings",
			failOn:   "ERROR",
			results:  []semgrep.Result{},
			wantFail: false,
		},
		{
			name:     "ERROR policy with WARNING only",
			failOn:   "ERROR",
			results:  []semgrep.Result{warningFinding},
			wantFail: false,
		},
		{
			name:     "ERROR policy with ERROR finding",
			failOn:   "ERROR",
			results:  []semgrep.Result{warningFinding, errorFinding},
			wantFail: true,
		},
		{
			name:     "WARNING policy with WARNING finding",
			failOn:   "WARNING",
			results:  []semgrep.Result{warningFinding},
			wantFail: true,
		},
		{
			name:     "WARN alias policy with ERROR finding",
			failOn:   "warn",
			results:  []semgrep.Result{errorFinding},
			wantFail: true,
		},
		{
			name:     "INFO policy with INFO finding",
			failOn:   "INFO",
			results:  []semgrep.Result{infoFinding},
			wantFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldFailBuild(tt.failOn, tt.results)
			if got != tt.wantFail {
				t.Errorf("expected shouldFailBuild=%v, got %v", tt.wantFail, got)
			}
		})
	}
}
