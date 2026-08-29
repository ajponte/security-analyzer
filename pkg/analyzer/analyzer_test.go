package analyzer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"security-analyzer/pkg/llm"
)

type mockToolClient struct {
	tools       []llm.Tool
	listErr     error
	callResults map[string]string
	callErr     error
	calledTools []string
}

func (m *mockToolClient) ListTools(ctx context.Context) ([]llm.Tool, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.tools, nil
}

func (m *mockToolClient) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (string, error) {
	m.calledTools = append(m.calledTools, toolName)
	if m.callErr != nil {
		return "", m.callErr
	}
	if res, ok := m.callResults[toolName]; ok {
		return res, nil
	}
	return "{}", nil
}

type mockLLMClient struct {
	responses []*llm.Response
	errors    []error
	callCount int
	history   [][]llm.Message
}

func (m *mockLLMClient) GenerateResponse(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	if m.callCount < len(m.errors) && m.errors[m.callCount] != nil {
		err := m.errors[m.callCount]
		m.callCount++
		return nil, err
	}

	if m.callCount >= len(m.responses) {
		return nil, errors.New("no more mock responses configured")
	}

	m.history = append(m.history, messages)
	resp := m.responses[m.callCount]
	m.callCount++
	return resp, nil
}

func TestAnalyzer_Analyze_DirectCompletion(t *testing.T) {
	tempDir := t.TempDir()
	reportFile := filepath.Join(tempDir, "test-report.md")

	mockTools := &mockToolClient{
		tools: []llm.Tool{{Name: "semgrep_scan", Description: "scan"}},
	}
	mockLLM := &mockLLMClient{
		responses: []*llm.Response{
			{
				Content: "# Security Audit Report\n\nNo issues found.",
			},
		},
	}

	az := NewAnalyzer(mockLLM, mockTools, Options{
		OutputFile: reportFile,
		MaxTurns:   5,
	})

	report, err := az.Analyze(context.Background(), ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report != "# Security Audit Report\n\nNo issues found." {
		t.Errorf("unexpected report content: %q", report)
	}

	// Verify file was written
	data, err := os.ReadFile(reportFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if string(data) != report {
		t.Errorf("file content %q does not match report %q", string(data), report)
	}
}

func TestAnalyzer_Analyze_WithToolCalls(t *testing.T) {
	mockTools := &mockToolClient{
		tools: []llm.Tool{{Name: "semgrep_scan", Description: "scan"}},
		callResults: map[string]string{
			"semgrep_scan": `{"results":[{"check_id":"vuln-1"}]}`,
		},
	}

	mockLLM := &mockLLMClient{
		responses: []*llm.Response{
			// Turn 1: tool call
			{
				Content: "Let me scan the code.",
				ToolCalls: []llm.ToolCall{
					{
						ID:        "call_1",
						Name:      "semgrep_scan",
						Arguments: `{"path":"."}`,
					},
				},
			},
			// Turn 2: final synthesis
			{
				Content: "Found 1 vulnerability.",
			},
		},
	}

	az := NewAnalyzer(mockLLM, mockTools, Options{
		OutputFile: "",
		MaxTurns:   5,
	})

	report, err := az.Analyze(context.Background(), ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report != "Found 1 vulnerability." {
		t.Errorf("unexpected report content: %q", report)
	}

	if len(mockTools.calledTools) != 1 || mockTools.calledTools[0] != "semgrep_scan" {
		t.Errorf("expected semgrep_scan tool call, got %v", mockTools.calledTools)
	}

	if mockLLM.callCount != 2 {
		t.Errorf("expected 2 LLM turns, got %d", mockLLM.callCount)
	}
}

func TestAnalyzer_Analyze_ToolDiscoveryError(t *testing.T) {
	mockTools := &mockToolClient{
		listErr: errors.New("MCP connection closed"),
	}
	mockLLM := &mockLLMClient{}

	az := NewAnalyzer(mockLLM, mockTools)
	_, err := az.Analyze(context.Background(), ".")
	if err == nil {
		t.Fatal("expected error on tool discovery failure, got nil")
	}
}

func TestAnalyzer_Analyze_LLMError(t *testing.T) {
	mockTools := &mockToolClient{
		tools: []llm.Tool{{Name: "semgrep_scan"}},
	}
	mockLLM := &mockLLMClient{
		errors: []error{errors.New("API rate limit exceeded")},
	}

	az := NewAnalyzer(mockLLM, mockTools)
	_, err := az.Analyze(context.Background(), ".")
	if err == nil {
		t.Fatal("expected error on LLM failure, got nil")
	}
}

func TestAnalyzer_Analyze_MaxTurnsExceeded(t *testing.T) {
	mockTools := &mockToolClient{
		tools: []llm.Tool{{Name: "semgrep_scan"}},
	}

	// Always returns tool call without text
	infiniteToolCalls := make([]*llm.Response, 5)
	for i := 0; i < 5; i++ {
		infiniteToolCalls[i] = &llm.Response{
			ToolCalls: []llm.ToolCall{
				{ID: "call_loop", Name: "semgrep_scan", Arguments: "{}"},
			},
		}
	}

	mockLLM := &mockLLMClient{
		responses: infiniteToolCalls,
	}

	az := NewAnalyzer(mockLLM, mockTools, Options{
		MaxTurns: 3,
	})

	_, err := az.Analyze(context.Background(), ".")
	if err == nil {
		t.Fatal("expected max iterations error, got nil")
	}
}

func TestAnalyzer_Analyze_MultipleToolCallsInOneTurn(t *testing.T) {
	mockTools := &mockToolClient{
		tools: []llm.Tool{{Name: "semgrep_scan", Description: "scan"}},
		callResults: map[string]string{
			"semgrep_scan": `{"results":[]}`,
		},
	}

	mockLLM := &mockLLMClient{
		responses: []*llm.Response{
			// Turn 1: 2 parallel tool calls
			{
				Content: "Scanning subdirectories",
				ToolCalls: []llm.ToolCall{
					{ID: "call_a", Name: "semgrep_scan", Arguments: `{"path":"./pkg"}`},
					{ID: "call_b", Name: "semgrep_scan", Arguments: `{"path":"./cmd"}`},
				},
			},
			// Turn 2: final report
			{
				Content: "Scan completed for all packages.",
			},
		},
	}

	az := NewAnalyzer(mockLLM, mockTools, Options{MaxTurns: 5})
	report, err := az.Analyze(context.Background(), ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report != "Scan completed for all packages." {
		t.Errorf("unexpected report: %q", report)
	}

	if len(mockTools.calledTools) != 2 {
		t.Fatalf("expected 2 tool executions, got %d (%v)", len(mockTools.calledTools), mockTools.calledTools)
	}
}

func TestAnalyzer_Analyze_OutputDirCreation(t *testing.T) {
	tempDir := t.TempDir()
	outDir := filepath.Join(tempDir, "custom-llm-reports")
	outFile := "security-findings.md"

	mockTools := &mockToolClient{
		tools: []llm.Tool{{Name: "semgrep_scan"}},
	}
	mockLLM := &mockLLMClient{
		responses: []*llm.Response{
			{Content: "Report inside directory"},
		},
	}

	az := NewAnalyzer(mockLLM, mockTools, Options{
		OutputDir:  outDir,
		OutputFile: outFile,
	})

	report, err := az.Analyze(context.Background(), ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedFile := filepath.Join(outDir, outFile)
	data, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("expected file to be written at %s: %v", expectedFile, err)
	}

	if string(data) != report {
		t.Errorf("file content %q does not match report %q", string(data), report)
	}
}

func TestAnalyzer_Analyze_NamedAfterScanID(t *testing.T) {
	tempDir := t.TempDir()
	outDir := filepath.Join(tempDir, "llm-reports")

	scanID := "scan-20260828-185800-abcdef12"
	mockTools := &mockToolClient{
		tools: []llm.Tool{{Name: "semgrep_scan"}},
		callResults: map[string]string{
			"semgrep_scan": fmt.Sprintf(`{"scan_id":"%s","results":[]}`, scanID),
		},
	}
	mockLLM := &mockLLMClient{
		responses: []*llm.Response{
			{
				Content: "Scanning codebase",
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "semgrep_scan", Arguments: `{"path":"."}`},
				},
			},
			{
				Content: "# Final Report\nAll clear.",
			},
		},
	}

	az := NewAnalyzer(mockLLM, mockTools, Options{
		OutputDir:  outDir,
		OutputFile: "llm-report.md", // default
	})

	report, err := az.Analyze(context.Background(), ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedFile := filepath.Join(outDir, fmt.Sprintf("%s.md", scanID))
	data, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("expected file to be written named after scan_id (%s): %v", expectedFile, err)
	}

	if string(data) != report {
		t.Errorf("file content %q does not match report %q", string(data), report)
	}
}
