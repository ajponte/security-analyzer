package gemini

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

func TestNewGeminiClient_MissingKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	if _, err := NewGeminiClient(); err == nil {
		t.Fatal("expected error when GEMINI_API_KEY is unset")
	}
}

func TestNewGeminiClient_DefaultsModel(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "mock-key")
	t.Setenv("LLM_MODEL", "")

	client, err := NewGeminiClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.model != defaultModel {
		t.Errorf("expected default model %q, got %q", defaultModel, client.model)
	}

	t.Setenv("LLM_MODEL", "gemini-custom")
	client, err = NewGeminiClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.model != "gemini-custom" {
		t.Errorf("expected gemini-custom, got %s", client.model)
	}
}

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	t.Setenv("GEMINI_API_KEY", "mock-key")
	t.Setenv("LLM_MODEL", "gemini-test")

	client, err := NewGeminiClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	client.SetEndpointBase(server.URL)
	client.SetHTTPClient(server.Client())
	return client, server
}

func TestGemini_GenerateResponse_TextOnly(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/gemini-test:generateContent") {
			t.Errorf("expected model in URL, got %q", r.URL.Path)
		}
		if r.URL.Query().Get("key") != "mock-key" {
			t.Errorf("expected key query param, got %q", r.URL.Query().Get("key"))
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"hello there"}]}}]}`))
	})

	resp, err := client.GenerateResponse(context.Background(), []llm.Message{
		{Role: llm.RoleSystem, Content: "sys"},
		{Role: llm.RoleUser, Content: "hi"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hello there" {
		t.Errorf("expected content 'hello there', got %q", resp.Content)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("expected no tool calls, got %d", len(resp.ToolCalls))
	}
}

func TestGemini_GenerateResponse_FunctionCall(t *testing.T) {
	var captured geminiRequest
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("decode: %v", err)
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[
			{"text":"planning"},
			{"functionCall":{"name":"semgrep_scan","args":{"path":"/tmp"}}}
		]}}]}`))
	})

	tools := []llm.Tool{{
		Name:        "semgrep_scan",
		Description: "scan",
		Parameters:  map[string]interface{}{"type": "object"},
	}}
	resp, err := client.GenerateResponse(context.Background(), []llm.Message{
		{Role: llm.RoleSystem, Content: "you are a scanner"},
		{Role: llm.RoleUser, Content: "scan please"},
	}, tools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.SystemInstruction == nil ||
		len(captured.SystemInstruction.Parts) != 1 ||
		captured.SystemInstruction.Parts[0].Text != "you are a scanner" {
		t.Errorf("expected systemInstruction to be set, got %+v", captured.SystemInstruction)
	}
	if len(captured.Tools) != 1 || len(captured.Tools[0].FunctionDeclarations) != 1 {
		t.Fatalf("expected one function declaration, got %+v", captured.Tools)
	}
	if captured.Tools[0].FunctionDeclarations[0].Name != "semgrep_scan" {
		t.Errorf("expected semgrep_scan declaration, got %+v", captured.Tools[0].FunctionDeclarations[0])
	}

	if resp.Content != "planning" {
		t.Errorf("expected content 'planning', got %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.Name != "semgrep_scan" {
		t.Errorf("expected tool name semgrep_scan, got %q", tc.Name)
	}
	if tc.ID == "" || !strings.HasPrefix(tc.ID, "call_") {
		t.Errorf("expected synthesized id, got %q", tc.ID)
	}
	if !strings.Contains(tc.Arguments, `"path":"/tmp"`) {
		t.Errorf("expected arguments JSON to contain path, got %q", tc.Arguments)
	}
}

func TestGemini_MessageTranslation_MultipleFunctionResponses(t *testing.T) {
	var captured geminiRequest
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("decode: %v", err)
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"completed"}]}}]}`))
	})

	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "you are a scanner"},
		{Role: llm.RoleUser, Content: "scan code"},
		{
			Role:    llm.RoleAssistant,
			Content: "calling two functions",
			ToolCalls: []llm.ToolCall{
				{ID: "call_1", Name: "semgrep_scan", Arguments: `{"path":"dir1"}`},
				{ID: "call_2", Name: "semgrep_scan", Arguments: `{"path":"dir2"}`},
			},
		},
		{
			Role:       llm.RoleTool,
			Content:    "res1",
			Name:       "semgrep_scan",
			ToolCallID: "call_1",
		},
		{
			Role:       llm.RoleTool,
			Content:    "res2",
			Name:       "semgrep_scan",
			ToolCallID: "call_2",
		},
	}

	if _, err := client.GenerateResponse(context.Background(), msgs, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must have 3 contents: user ("scan code"), model (call_1 + call_2), user (res1 + res2)
	if len(captured.Contents) != 3 {
		t.Fatalf("expected 3 contents (user/model/user), got %d", len(captured.Contents))
	}

	toolTurn := captured.Contents[2]
	if toolTurn.Role != "user" {
		t.Errorf("expected user role for tool responses turn, got %q", toolTurn.Role)
	}
	if len(toolTurn.Parts) != 2 {
		t.Fatalf("expected 2 functionResponse parts in single user turn, got %d", len(toolTurn.Parts))
	}
	if toolTurn.Parts[0].FunctionResponse == nil || toolTurn.Parts[1].FunctionResponse == nil {
		t.Fatalf("expected both parts to contain function responses, got %+v", toolTurn.Parts)
	}
	if toolTurn.Parts[0].FunctionResponse.Name != "semgrep_scan" || toolTurn.Parts[1].FunctionResponse.Name != "semgrep_scan" {
		t.Errorf("unexpected function response names: %+v", toolTurn.Parts)
	}
}
