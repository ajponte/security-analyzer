package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"security-analyzer/pkg/config"
	"security-analyzer/pkg/scanner/semgrep"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ScanArguments holds the input parameters for the semgrep_scan tool.
type ScanArguments struct {
	Path  string `json:"path" jsonschema:"Absolute or relative path to the directory or repository that needs to be scanned"`
	Rules string `json:"rules,omitempty" jsonschema:"A comma-separated list of rule names or paths (e.g. p/default,p/golang). If omitted, uses default config"`
}

// isSafePath checks if targetPath is within the allowedWorkspace directory.
func isSafePath(allowedWorkspace, targetPath string) (bool, error) {
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return false, fmt.Errorf("failed to resolve absolute path of target: %w", err)
	}

	workspaceAbs, err := filepath.Abs(allowedWorkspace)
	if err != nil {
		return false, fmt.Errorf("failed to resolve absolute path of workspace: %w", err)
	}

	rel, err := filepath.Rel(workspaceAbs, targetAbs)
	if err != nil {
		return false, fmt.Errorf("failed to get relative path: %w", err)
	}

	if strings.HasPrefix(rel, "..") {
		return false, nil
	}

	return true, nil
}

// registerSemgrepScanTool adds the semgrep_scan tool to the MCP server.
func registerSemgrepScanTool(server *mcp.Server, cfg *config.SemgrepConfig, allowedWorkspace string) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "semgrep_scan",
		Description: "Triggers a local Semgrep scan against a specific directory and returns a structured list of security findings.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ScanArguments) (*mcp.CallToolResult, any, error) {
		// Verify directory traversal safety.
		safe, err := isSafePath(allowedWorkspace, args.Path)
		if err != nil {
			return nil, nil, fmt.Errorf("failed path safety validation: %w", err)
		}
		if !safe {
			return nil, nil, fmt.Errorf("security error: path %q is outside the allowed workspace %q", args.Path, allowedWorkspace)
		}

		targetAbs, err := filepath.Abs(args.Path)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to resolve absolute path of target: %w", err)
		}

		// Configure scanner options dynamically.
		scanCfg := *cfg
		if args.Rules != "" {
			scanCfg.Rules = args.Rules
		}

		// Execute Semgrep scan.
		scanner := semgrep.NewSemgrepScanner(&scanCfg)
		report, err := scanner.Scan(ctx, targetAbs)
		if err != nil {
			return nil, nil, fmt.Errorf("scan failed: %w", err)
		}

		// Return report as JSON.
		reportJSON, err := json.Marshal(report)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal scan report: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(reportJSON),
				},
			},
		}, nil, nil
	})
}
