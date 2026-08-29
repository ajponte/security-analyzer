package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"security-analyzer/pkg/llm"
)

func TestNewAnthropicClient_MissingKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	if _, err := NewAnthropicClient(); err == nil {
		t.Fatal("expected error when ANTHROPIC_API_KEY is unset")
	}
}

func TestNewAnthropicClient_DefaultsModel(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "mock-key")
	t.Setenv("LLM_MODEL", "")

	client, err := NewAnthropicClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.model != defaultModel {
		t.Errorf("expected model %q, got %q", defaultModel, client.model)
	}

	t.Setenv("LLM_MODEL", "claude-custom")
	client, err = NewAnthropicClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.model != "claude-custom" {
		t.Errorf("expected model claude-custom, got %s", client.model)
	}
}

// newTestClient wires the client to a test server.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	t.Setenv("ANTHROPIC_API_KEY", "mock-key")
	t.Setenv("LLM_MODEL", "claude-test-model")

	client, err := NewAnthropicClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	client.SetEndpoint(server.URL)
	client.SetHTTPClient(server.Client())
	return client, server
}

func TestAnthropic_GenerateResponse_TextOnly(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "mock-key" {
			t.Errorf("expected x-api-key header 'mock-key', got %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != anthropicVersion {
			t.Errorf("expected anthropic-version header %q, got %q", anthropicVersion, got)
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"hello world"}]}`))
	})

	resp, err := client.GenerateResponse(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "hi"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hello world" {
		t.Errorf("expected content 'hello world', got %q", resp.Content)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("expected no tool calls, got %d", len(resp.ToolCalls))
	}
}

func TestAnthropic_GenerateResponse_ToolCall(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req anthropicRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.System != "sys" {
			t.Errorf("expected system 'sys', got %q", req.System)
		}
		if len(req.Tools) != 1 || req.Tools[0].Name != "semgrep_scan" {
			t.Errorf("expected semgrep_scan tool, got %+v", req.Tools)
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"content":[
			{"type":"text","text":"scanning..."},
			{"type":"tool_use","id":"toolu_1","name":"semgrep_scan","input":{"path":"/tmp/src"}}
		]}`))
	})

	tools := []llm.Tool{{
		Name:        "semgrep_scan",
		Description: "scan",
		Parameters:  map[string]interface{}{"type": "object"},
	}}
	resp, err := client.GenerateResponse(context.Background(), []llm.Message{
		{Role: llm.RoleSystem, Content: "sys"},
		{Role: llm.RoleUser, Content: "scan it"},
	}, tools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "scanning..." {
		t.Errorf("expected content 'scanning...', got %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "toolu_1" || tc.Name != "semgrep_scan" {
		t.Errorf("unexpected tool call: %+v", tc)
	}
	if !strings.Contains(tc.Arguments, `"path":"/tmp/src"`) {
		t.Errorf("expected arguments to contain path, got %q", tc.Arguments)
	}
}

func TestAnthropic_MessageTranslation_ToolResult(t *testing.T) {
	var captured anthropicRequest
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("decode: %v", err)
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	})

	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "you are a scanner"},
		{Role: llm.RoleUser, Content: "please scan"},
		{
			Role:    llm.RoleAssistant,
			Content: "calling tool",
			ToolCalls: []llm.ToolCall{{
				ID:        "toolu_abc",
				Name:      "semgrep_scan",
				Arguments: `{"path":"."}`,
			}},
		},
		{
			Role:       llm.RoleTool,
			Content:    "findings: none",
			Name:       "semgrep_scan",
			ToolCallID: "toolu_abc",
		},
	}
	if _, err := client.GenerateResponse(context.Background(), msgs, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.System != "you are a scanner" {
		t.Errorf("expected system to be set, got %q", captured.System)
	}
	if len(captured.Messages) != 3 {
		t.Fatalf("expected 3 wire messages (user/assistant/user), got %d", len(captured.Messages))
	}

	assistant := captured.Messages[1]
	if assistant.Role != "assistant" {
		t.Errorf("expected assistant role, got %q", assistant.Role)
	}
	var sawToolUse bool
	for _, c := range assistant.Content {
		if c.Type == "tool_use" && c.ID == "toolu_abc" && c.Name == "semgrep_scan" {
			sawToolUse = true
		}
	}
	if !sawToolUse {
		t.Errorf("expected assistant message to contain tool_use, got %+v", assistant.Content)
	}

	toolResp := captured.Messages[2]
	if toolResp.Role != "user" {
		t.Errorf("expected tool result to be sent as user role, got %q", toolResp.Role)
	}
	if len(toolResp.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(toolResp.Content))
	}
	block := toolResp.Content[0]
	if block.Type != "tool_result" {
		t.Errorf("expected type tool_result, got %q", block.Type)
	}
	if block.ToolUseID != "toolu_abc" {
		t.Errorf("expected tool_use_id 'toolu_abc', got %q", block.ToolUseID)
	}
	if block.Content != "findings: none" {
		t.Errorf("expected tool result content, got %q", block.Content)
	}
}

func TestAnthropic_MessageTranslation_MultipleToolResults(t *testing.T) {
	var captured anthropicRequest
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("decode: %v", err)
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"done"}]}`))
	})

	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "you are a scanner"},
		{Role: llm.RoleUser, Content: "please scan"},
		{
			Role:    llm.RoleAssistant,
			Content: "calling 2 tools",
			ToolCalls: []llm.ToolCall{
				{ID: "toolu_1", Name: "semgrep_scan", Arguments: `{"path":"dir1"}`},
				{ID: "toolu_2", Name: "semgrep_scan", Arguments: `{"path":"dir2"}`},
			},
		},
		{
			Role:       llm.RoleTool,
			Content:    "findings 1",
			Name:       "semgrep_scan",
			ToolCallID: "toolu_1",
		},
		{
			Role:       llm.RoleTool,
			Content:    "findings 2",
			Name:       "semgrep_scan",
			ToolCallID: "toolu_2",
		},
	}
	if _, err := client.GenerateResponse(context.Background(), msgs, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must have 3 wire messages: user, assistant, user (with 2 tool_result blocks)
	if len(captured.Messages) != 3 {
		t.Fatalf("expected 3 wire messages (user/assistant/user), got %d", len(captured.Messages))
	}

	toolTurn := captured.Messages[2]
	if toolTurn.Role != "user" {
		t.Errorf("expected user role for tool results turn, got %q", toolTurn.Role)
	}
	if len(toolTurn.Content) != 2 {
		t.Fatalf("expected 2 tool_result content blocks in single user message, got %d", len(toolTurn.Content))
	}
	if toolTurn.Content[0].ToolUseID != "toolu_1" || toolTurn.Content[1].ToolUseID != "toolu_2" {
		t.Errorf("unexpected tool results content: %+v", toolTurn.Content)
	}
}
