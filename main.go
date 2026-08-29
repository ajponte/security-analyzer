package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"security-analyzer/pkg/analyzer"
	"security-analyzer/pkg/config"
	"security-analyzer/pkg/llm/factory"
	"security-analyzer/pkg/mcp"
	"security-analyzer/pkg/report"
	"security-analyzer/pkg/scanner/semgrep"
)

type runMode string

const (
	modeMCP     runMode = "mcp"
	modeAnalyze runMode = "analyze"
	modeScan    runMode = "scan"
)

type cliOptions struct {
	mode     runMode
	scanPath string
}

func parseCLIArgs(args []string) cliOptions {
	opts := cliOptions{
		mode:     modeScan,
		scanPath: ".",
	}

	if len(args) <= 1 {
		return opts
	}

	switch args[1] {
	case "mcp":
		opts.mode = modeMCP
		if len(args) > 2 {
			opts.scanPath = args[2]
		}
	case "analyze":
		opts.mode = modeAnalyze
		if len(args) > 2 {
			opts.scanPath = args[2]
		}
	case "scan":
		opts.mode = modeScan
		if len(args) > 2 {
			opts.scanPath = args[2]
		}
	default:
		opts.scanPath = args[1]
	}

	return opts
}

func main() {
	opts := parseCLIArgs(os.Args)

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	var runErr error
	switch opts.mode {
	case modeMCP:
		runErr = runMCP(ctx, &cfg.Semgrep, opts.scanPath)
	case modeAnalyze:
		runErr = runAnalyze(ctx, cfg, opts.scanPath)
	case modeScan:
		runErr = runScan(ctx, &cfg.Semgrep, opts.scanPath)
	}

	if runErr != nil {
		slog.Error("Execution failed", "error", runErr)
		os.Exit(1)
	}
}

func runMCP(ctx context.Context, cfg *config.SemgrepConfig, workspace string) error {
	server := mcp.NewServer(cfg, workspace)
	if err := server.Start(ctx); err != nil {
		return fmt.Errorf("MCP server terminated: %w", err)
	}
	return nil
}

func runAnalyze(ctx context.Context, cfg *config.Config, scanPath string) error {
	setupLogger()

	absPath, err := filepath.Abs(scanPath)
	if err != nil {
		absPath = scanPath
	}

	slog.Info("Starting LLM-driven security analysis",
		"path", absPath,
		"provider", cfg.LLM.Provider,
		"model", cfg.LLM.Model)

	llmClient, err := factory.NewClient(&cfg.LLM)
	if err != nil {
		return fmt.Errorf("initializing LLM client: %w", err)
	}

	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating executable path: %w", err)
	}

	mcpClient, err := mcp.NewMCPClient(selfPath, absPath)
	if err != nil {
		return fmt.Errorf("initializing MCP client: %w", err)
	}
	defer func() {
		_ = mcpClient.Close()
	}()

	az := analyzer.NewAnalyzer(llmClient, mcpClient)
	reportContent, err := az.Analyze(ctx, absPath)
	if err != nil {
		return fmt.Errorf("running security analysis: %w", err)
	}

	fmt.Println(reportContent)
	return nil
}

func runScan(ctx context.Context, cfg *config.SemgrepConfig, scanPath string) error {
	setupLogger()
	slog.Info("Starting Semgrep security scan", "path", scanPath)

	if !cfg.SemgrepExists {
		return fmt.Errorf("semgrep CLI is not installed or not found in system PATH. Please install semgrep before scanning")
	}

	scanner := semgrep.NewSemgrepScanner(cfg)
	reportData, err := scanner.Scan(ctx, scanPath)
	if err != nil {
		return fmt.Errorf("running semgrep scan: %w", err)
	}

	slog.Info("Scan completed successfully",
		"findings", len(reportData.Results),
		"scanned_files", len(reportData.Paths.Scanned),
		"errors", len(reportData.Errors))

	mdReporter := report.NewMarkdownReporter("report.md")
	if err := mdReporter.WriteReport(reportData); err != nil {
		slog.Error("Failed to write Markdown report", "error", err)
	} else {
		slog.Info("Vulnerability markdown report written to report.md")
	}

	ghReporter := report.NewGitHubReporter()
	if err := ghReporter.WriteReport(reportData); err != nil {
		slog.Error("Failed to write GitHub summary", "error", err)
	}

	if shouldFailBuild(cfg.FailOn, reportData.Results) {
		return fmt.Errorf("build failure: Semgrep findings violate the build failure policy (%s)", strings.ToUpper(cfg.FailOn))
	}

	slog.Info("Build check passed.")
	return nil
}

func shouldFailBuild(failOn string, results []semgrep.Result) bool {
	failPolicy := strings.ToUpper(strings.TrimSpace(failOn))

	for _, res := range results {
		sev := strings.ToUpper(res.Extra.Severity)
		switch failPolicy {
		case "ERROR":
			if sev == "ERROR" {
				return true
			}
		case "WARNING", "WARN":
			if sev == "ERROR" || sev == "WARNING" || sev == "WARN" {
				return true
			}
		case "INFO":
			if sev == "ERROR" || sev == "WARNING" || sev == "WARN" || sev == "INFO" {
				return true
			}
		}
	}
	return false
}

func setupLogger() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
}
