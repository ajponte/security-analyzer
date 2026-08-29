package openai

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

func TestNewOpenAIClient(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	_, err := NewOpenAIClient()
	if err == nil {
		t.Error("expected error when OPENAI_API_KEY is not set")
	}

	t.Setenv("OPENAI_API_KEY", "mock-key")
	t.Setenv("LLM_MODEL", "")
	client, err := NewOpenAIClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.model != defaultModel {
		t.Errorf("expected default model %s, got %s", defaultModel, client.model)
	}

	t.Setenv("LLM_MODEL", "gpt-4o-custom")
	client, err = NewOpenAIClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.model != "gpt-4o-custom" {
		t.Errorf("expected model gpt-4o-custom, got %s", client.model)
	}
}

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := NewClientWithConfig("mock-key", "gpt-4o-mini",
		WithBaseURL(server.URL+"/v1"),
		WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return client, server
}

func TestOpenAI_Options(t *testing.T) {
	customHTTP := &http.Client{}
	client, err := NewClientWithConfig("mock-key", "custom-model",
		WithBaseURL("https://custom.api.com/v1"),
		WithHTTPClient(customHTTP),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.config.BaseURL != "https://custom.api.com/v1" {
		t.Errorf("expected custom BaseURL, got %q", client.config.BaseURL)
	}
	if client.config.HTTPClient != customHTTP {
		t.Errorf("expected custom HTTPClient")
	}
}

func TestOpenAI_GenerateResponse_TextOnly(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("expected path to end with /chat/completions, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer mock-key" {
			t.Errorf("expected Authorization header 'Bearer mock-key', got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"id": "chatcmpl-test",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hello from OpenAI",
					},
					"finish_reason": "stop",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	resp, err := client.GenerateResponse(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "Hello"},
	}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello from OpenAI" {
		t.Errorf("expected content 'Hello from OpenAI', got %q", resp.Content)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("expected no tool calls, got %d", len(resp.ToolCalls))
	}
}

func TestOpenAI_GenerateResponse_ToolCall(t *testing.T) {
	var receivedBody map[string]interface{}
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &receivedBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"id": "chatcmpl-tool",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Scanning repository...",
						"tool_calls": []map[string]interface{}{
							{
								"id":   "call_abc123",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "semgrep_scan",
									"arguments": `{"path":"."}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	tools := []llm.Tool{
		{
			Name:        "semgrep_scan",
			Description: "Run scan",
			Parameters:  map[string]interface{}{"type": "object"},
		},
	}

	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "You are a security scanner."},
		{Role: llm.RoleUser, Content: "Scan current repo."},
	}

	resp, err := client.GenerateResponse(context.Background(), messages, tools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Scanning repository..." {
		t.Errorf("expected content 'Scanning repository...', got %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "call_abc123" {
		t.Errorf("expected ID 'call_abc123', got %q", tc.ID)
	}
	if tc.Name != "semgrep_scan" {
		t.Errorf("expected tool name 'semgrep_scan', got %q", tc.Name)
	}
	if tc.Arguments != `{"path":"."}` {
		t.Errorf("expected arguments '{\"path\":\".\"}', got %q", tc.Arguments)
	}

	// Verify tools were sent in request
	rawTools, ok := receivedBody["tools"].([]interface{})
	if !ok || len(rawTools) != 1 {
		t.Fatalf("expected 1 tool in wire request, got %v", receivedBody["tools"])
	}
}

func TestOpenAI_GenerateResponse_MultiTurnToolResult(t *testing.T) {
	var receivedBody map[string]interface{}
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &receivedBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"id": "chatcmpl-final",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "No vulnerabilities found.",
					},
					"finish_reason": "stop",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "You are a scanner."},
		{Role: llm.RoleUser, Content: "Scan code."},
		{
			Role:      llm.RoleAssistant,
			Content:   "",
			ToolCalls: []llm.ToolCall{{ID: "call_999", Name: "semgrep_scan", Arguments: `{"path":"."}`}},
		},
		{
			Role:       llm.RoleTool,
			Content:    `{"results":[]}`,
			ToolCallID: "call_999",
		},
	}

	resp, err := client.GenerateResponse(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "No vulnerabilities found." {
		t.Errorf("expected content 'No vulnerabilities found.', got %q", resp.Content)
	}

	// Verify wire messages
	msgs, ok := receivedBody["messages"].([]interface{})
	if !ok || len(msgs) != 4 {
		t.Fatalf("expected 4 wire messages, got %v", receivedBody["messages"])
	}

	toolMsg := msgs[3].(map[string]interface{})
	if toolMsg["role"] != "tool" {
		t.Errorf("expected role 'tool', got %v", toolMsg["role"])
	}
	if toolMsg["tool_call_id"] != "call_999" {
		t.Errorf("expected tool_call_id 'call_999', got %v", toolMsg["tool_call_id"])
	}
	if toolMsg["content"] != `{"results":[]}` {
		t.Errorf("expected content '{\"results\":[]}', got %v", toolMsg["content"])
	}
}

func TestOpenAI_GenerateResponse_APIError(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Incorrect API key provided"}}`))
	})

	_, err := client.GenerateResponse(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "Hi"},
	}, nil)

	if err == nil {
		t.Fatal("expected error on 401 response, got nil")
	}
}
