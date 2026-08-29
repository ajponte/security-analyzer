package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"security-analyzer/pkg/llm"
)

const (
	defaultMaxTurns   = 10
	defaultReportDir  = "llm-reports"
	defaultReportFile = "llm-report.md"
	systemPrompt      = "You are an expert security researcher. Scan the codebase using the 'semgrep_scan' tool, analyze the findings, and generate a comprehensive security report detailing findings grouped by severity (Critical, High, Medium, Low) with code snippets and recommendations."
)

// Options configures the analysis execution.
type Options struct {
	MaxTurns   int
	OutputDir  string
	OutputFile string
}

// ToolClient defines the tool discovery and execution operations provided by an MCP client.
type ToolClient interface {
	ListTools(ctx context.Context) ([]llm.Tool, error)
	CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (string, error)
}

// Analyzer orchestrates the LLM and MCP tool execution to generate security reports.
type Analyzer struct {
	llmClient      llm.LLMClient
	toolClient     ToolClient
	opts           Options
	capturedScanID string
}

// NewAnalyzer creates a new Analyzer instance.
func NewAnalyzer(llmClient llm.LLMClient, toolClient ToolClient, opts ...Options) *Analyzer {
	opt := Options{
		MaxTurns:   defaultMaxTurns,
		OutputDir:  defaultReportDir,
		OutputFile: defaultReportFile,
	}
	if len(opts) > 0 {
		if opts[0].MaxTurns > 0 {
			opt.MaxTurns = opts[0].MaxTurns
		}
		opt.OutputDir = opts[0].OutputDir
		opt.OutputFile = opts[0].OutputFile
	}

	return &Analyzer{
		llmClient:  llmClient,
		toolClient: toolClient,
		opts:       opt,
	}
}

// Analyze runs the agentic loop to scan the target path and returns the generated report.
func (a *Analyzer) Analyze(ctx context.Context, scanPath string) (string, error) {
	tools, err := a.toolClient.ListTools(ctx)
	if err != nil {
		return "", fmt.Errorf("discovering tools from MCP server: %w", err)
	}
	slog.Info("Discovered tools from MCP server", "count", len(tools))

	messages := a.initialMessages(scanPath)

	for turn := 0; turn < a.opts.MaxTurns; turn++ {
		slog.Info("Sending conversation context to LLM", "turn", turn+1)
		resp, err := a.llmClient.GenerateResponse(ctx, messages, tools)
		if err != nil {
			return "", fmt.Errorf("LLM turn %d failed: %w", turn+1, err)
		}

		messages = append(messages, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Concluded when no further tool calls are requested.
		if len(resp.ToolCalls) == 0 {
			a.saveReport(resp.Content)
			return resp.Content, nil
		}

		// Execute tool calls and fail early on execution errors.
		toolMessages, err := a.executeToolCalls(ctx, resp.ToolCalls)
		if err != nil {
			return "", fmt.Errorf("tool execution failed: %w", err)
		}
		messages = append(messages, toolMessages...)
	}

	return "", fmt.Errorf("analysis did not finish within %d iterations", a.opts.MaxTurns)
}

func (a *Analyzer) initialMessages(scanPath string) []llm.Message {
	return []llm.Message{
		{
			Role:    llm.RoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    llm.RoleUser,
			Content: fmt.Sprintf("Analyze code in %s", scanPath),
		},
	}
}

func (a *Analyzer) executeToolCalls(ctx context.Context, toolCalls []llm.ToolCall) ([]llm.Message, error) {
	results := make([]llm.Message, 0, len(toolCalls))
	for _, tc := range toolCalls {
		slog.Info("LLM requested tool call", "tool", tc.Name, "id", tc.ID)

		args, err := parseToolArguments(tc.Arguments)
		if err != nil {
			return nil, fmt.Errorf("parsing tool arguments for %s: %w", tc.Name, err)
		}

		toolResult, err := a.toolClient.CallTool(ctx, tc.Name, args)
		if err != nil {
			return nil, fmt.Errorf("calling tool %s: %w", tc.Name, err)
		}

		// Extract scan_id if returned in the tool result
		var probe struct {
			ScanID string `json:"scan_id"`
		}
		if probeErr := json.Unmarshal([]byte(toolResult), &probe); probeErr == nil && probe.ScanID != "" {
			a.capturedScanID = probe.ScanID
		}

		results = append(results, llm.Message{
			Role:       llm.RoleTool,
			Content:    toolResult,
			Name:       tc.Name,
			ToolCallID: tc.ID,
		})
	}
	return results, nil
}

func parseToolArguments(argsJSON string) (map[string]interface{}, error) {
	if argsJSON == "" {
		return nil, nil
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil, err
	}
	return args, nil
}

func (a *Analyzer) saveReport(reportContent string) {
	outputFile := a.opts.OutputFile
	if outputFile == "" {
		return
	}

	// If using the default report name and a scan_id was captured, name file after the scan ID.
	if a.capturedScanID != "" && outputFile == defaultReportFile {
		outputFile = fmt.Sprintf("%s.md", a.capturedScanID)
	}

	targetPath := outputFile
	if a.opts.OutputDir != "" && !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(a.opts.OutputDir, targetPath)
	}

	dir := filepath.Dir(targetPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			slog.Error("Failed to create report directory", "dir", dir, "error", err)
			return
		}
	}

	if err := os.WriteFile(targetPath, []byte(reportContent), 0644); err != nil {
		slog.Error("Failed to write output report file", "file", targetPath, "error", err)
	} else {
		slog.Info("Security analysis report saved", "file", targetPath)
	}
}
