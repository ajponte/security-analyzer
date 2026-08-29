package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"security-analyzer/pkg/llm"
)

const (
	defaultMaxTurns   = 10
	defaultReportFile = "llm-report.md"
	systemPrompt      = "You are an expert security researcher. Scan the codebase using the 'semgrep_scan' tool, analyze the findings, and generate a comprehensive security report detailing findings grouped by severity (Critical, High, Medium, Low) with code snippets and recommendations."
)

// Options configures the analysis execution.
type Options struct {
	MaxTurns   int
	OutputFile string
}

// ToolClient defines the tool discovery and execution operations provided by an MCP client.
type ToolClient interface {
	ListTools(ctx context.Context) ([]llm.Tool, error)
	CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (string, error)
}

// Analyzer orchestrates the LLM and MCP tool execution to generate security reports.
type Analyzer struct {
	llmClient  llm.LLMClient
	toolClient ToolClient
	opts       Options
}

// NewAnalyzer creates a new Analyzer instance.
func NewAnalyzer(llmClient llm.LLMClient, toolClient ToolClient, opts ...Options) *Analyzer {
	opt := Options{
		MaxTurns:   defaultMaxTurns,
		OutputFile: defaultReportFile,
	}
	if len(opts) > 0 {
		if opts[0].MaxTurns > 0 {
			opt.MaxTurns = opts[0].MaxTurns
		}
		if opts[0].OutputFile != "" {
			opt.OutputFile = opts[0].OutputFile
		}
	}

	return &Analyzer{
		llmClient:  llmClient,
		toolClient: toolClient,
		opts:       opt,
	}
}

// Analyze runs the agentic loop to scan the target path and returns the generated report.
func (a *Analyzer) Analyze(ctx context.Context, scanPath string) (string, error) {
	// 1. Discover tools dynamically from the MCP server.
	tools, err := a.toolClient.ListTools(ctx)
	if err != nil {
		return "", fmt.Errorf("discovering tools from MCP server: %w", err)
	}

	slog.Info("Discovered tools from MCP server", "count", len(tools))

	// 2. Prepare initial messages.
	messages := []llm.Message{
		{
			Role:    llm.RoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    llm.RoleUser,
			Content: fmt.Sprintf("Analyze code in %s", scanPath),
		},
	}

	var finalReport string

	// 3. Multi-turn execution loop.
	for turn := 0; turn < a.opts.MaxTurns; turn++ {
		slog.Info("Sending conversation context to LLM", "turn", turn+1)
		resp, err := a.llmClient.GenerateResponse(ctx, messages, tools)
		if err != nil {
			return "", fmt.Errorf("LLM turn %d failed: %w", turn+1, err)
		}

		// Record assistant message.
		messages = append(messages, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Check if LLM has concluded without requesting more tools.
		if len(resp.ToolCalls) == 0 {
			finalReport = resp.Content
			break
		}

		// Execute tool calls.
		for _, tc := range resp.ToolCalls {
			slog.Info("LLM requested tool call", "tool", tc.Name, "id", tc.ID)

			var args map[string]interface{}
			if tc.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
					errMsg := fmt.Sprintf("Error parsing tool arguments: %v", err)
					messages = append(messages, llm.Message{
						Role:       llm.RoleTool,
						Content:    errMsg,
						Name:       tc.Name,
						ToolCallID: tc.ID,
					})
					continue
				}
			}

			// Call MCP server.
			toolResult, err := a.toolClient.CallTool(ctx, tc.Name, args)
			if err != nil {
				toolResult = fmt.Sprintf("Tool execution failed: %v", err)
			}

			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				Content:    toolResult,
				Name:       tc.Name,
				ToolCallID: tc.ID,
			})
		}
	}

	if finalReport == "" {
		return "", fmt.Errorf("analysis did not finish within %d iterations", a.opts.MaxTurns)
	}

	// 4. Save report file if configured.
	if a.opts.OutputFile != "" {
		if err := os.WriteFile(a.opts.OutputFile, []byte(finalReport), 0644); err != nil {
			slog.Error("Failed to write output report file", "file", a.opts.OutputFile, "error", err)
		} else {
			slog.Info("Security analysis report saved", "file", a.opts.OutputFile)
		}
	}

	return finalReport, nil
}
