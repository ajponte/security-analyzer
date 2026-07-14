package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"security-analyzer/pkg/config"
	"security-analyzer/pkg/mcp"
	"security-analyzer/pkg/report"
	"security-analyzer/pkg/scanner/semgrep"
)

func main() {
	// 1. Determine run mode (MCP or local scan).
	isMCP := false
	scanPath := "."

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "mcp":
			isMCP = true
		case "scan":
			if len(os.Args) > 2 {
				scanPath = os.Args[2]
			}
		default:
			// If not a recognized subcommand, treat the argument as the scan path.
			scanPath = os.Args[1]
		}
	}

	// 2. Load Configuration.
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// 3. Execution based on mode.
	if isMCP {
		// Run in MCP mode.
		server := mcp.NewServer(cfg)
		ctx := context.Background()
		if err := server.Start(ctx); err != nil {
			slog.Error("MCP server terminated with error", "error", err)
			os.Exit(1)
		}
	} else {
		// Set up standard CLI logger to output to os.Stdout.
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
		slog.SetDefault(logger)

		slog.Info("Starting Semgrep security scan", "path", scanPath)

		if !cfg.SemgrepExists {
			slog.Error("semgrep CLI is not installed or not found in system PATH. Please install semgrep before scanning.")
			os.Exit(1)
		}

		// Run local scan.
		scanner := semgrep.NewSemgrepScanner(cfg)
		ctx := context.Background()
		reportData, err := scanner.Scan(ctx, scanPath)
		if err != nil {
			slog.Error("Scan failed", "error", err)
			os.Exit(1)
		}

		slog.Info("Scan completed successfully",
			"findings", len(reportData.Results),
			"scanned_files", len(reportData.Paths.Scanned),
			"errors", len(reportData.Errors))

		// Write Markdown report (defaulting to report.md).
		mdReporter := report.NewMarkdownReporter("report.md")
		if err := mdReporter.WriteReport(reportData); err != nil {
			slog.Error("Failed to write Markdown report", "error", err)
		} else {
			slog.Info("Vulnerability markdown report written to report.md")
		}

		// Write GitHub Step Summary if applicable.
		ghReporter := report.NewGitHubReporter()
		if err := ghReporter.WriteReport(reportData); err != nil {
			slog.Error("Failed to write GitHub summary", "error", err)
		}

		// Check build failure policy.
		shouldFail := false
		failOn := strings.ToUpper(cfg.FailOn)

		for _, res := range reportData.Results {
			severity := strings.ToUpper(res.Extra.Severity)
			switch failOn {
			case "ERROR":
				if severity == "ERROR" {
					shouldFail = true
				}
			case "WARNING", "WARN":
				if severity == "ERROR" || severity == "WARNING" || severity == "WARN" {
					shouldFail = true
				}
			case "INFO":
				if severity == "ERROR" || severity == "WARNING" || severity == "WARN" || severity == "INFO" {
					shouldFail = true
				}
			}
			if shouldFail {
				break
			}
		}

		if shouldFail {
			slog.Error("Build failure: Semgrep findings violate the build failure policy", "policy", failOn)
			os.Exit(1)
		}

		slog.Info("Build check passed.")
	}
}
