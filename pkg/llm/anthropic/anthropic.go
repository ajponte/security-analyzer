// Package anthropic provides an llm.LLMClient implementation backed by
// Anthropic's Messages API.
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"security-analyzer/pkg/llm"
)

const (
	defaultModel     = "claude-3-5-sonnet-latest"
	defaultEndpoint  = "https://api.anthropic.com/v1/messages"
	anthropicVersion = "2023-06-01"
	defaultMaxTokens = 4096
)

// httpDoer is the minimal HTTP interface used by the client. It allows tests
// to inject a fake transport without hitting the network.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client implements llm.LLMClient for Anthropic Claude models.
type Client struct {
	apiKey    string
	model     string
	endpoint  string
	maxTokens int
	http      httpDoer
}

// NewAnthropicClient constructs a Client from environment configuration.
func NewAnthropicClient() (*Client, error) {
	return NewAnthropicClientWithConfig(os.Getenv("ANTHROPIC_API_KEY"), os.Getenv("LLM_MODEL"))
}

// NewAnthropicClientWithConfig constructs a Client with explicit API key and model.
func NewAnthropicClientWithConfig(apiKey, model string) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("missing API key for Anthropic")
	}

	if model == "" {
		model = defaultModel
	}

	return &Client{
		apiKey:    apiKey,
		model:     model,
		endpoint:  defaultEndpoint,
		maxTokens: defaultMaxTokens,
		http:      http.DefaultClient,
	}, nil
}

// SetHTTPClient overrides the HTTP transport used by the client. Intended for
// tests.
func (c *Client) SetHTTPClient(doer httpDoer) {
	if doer != nil {
		c.http = doer
	}
}

// SetEndpoint overrides the Messages API endpoint. Intended for tests.
func (c *Client) SetEndpoint(endpoint string) {
	if endpoint != "" {
		c.endpoint = endpoint
	}
}

// --- Anthropic wire format ---

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
}

type anthropicContent struct {
	Type string `json:"type"`

	// text
	Text string `json:"text,omitempty"`

	// tool_use (assistant)
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result (user)
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

type anthropicTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"input_schema"`
}

type anthropicResponse struct {
	Content []anthropicResponseBlock `json:"content"`
	Error   *anthropicError          `json:"error,omitempty"`
}

type anthropicResponseBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// GenerateResponse implements llm.LLMClient.
func (c *Client) GenerateResponse(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	system, converted, err := translateMessages(messages)
	if err != nil {
		return nil, fmt.Errorf("translating messages: %w", err)
	}

	reqBody := anthropicRequest{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		System:    system,
		Messages:  converted,
		Tools:     translateTools(tools),
	}

	buf, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("anthropic API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	var parsed anthropicResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("anthropic API error: %s: %s", parsed.Error.Type, parsed.Error.Message)
	}

	out := &llm.Response{}
	for _, block := range parsed.Content {
		switch block.Type {
		case "text":
			if out.Content != "" {
				out.Content += "\n"
			}
			out.Content += block.Text
		case "tool_use":
			args := string(block.Input)
			if args == "" {
				args = "{}"
			}
			out.ToolCalls = append(out.ToolCalls, llm.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: args,
			})
		}
	}

	return out, nil
}

// translateMessages converts llm.Message entries into Anthropic's format. The
// system prompt is returned separately (Anthropic accepts it as a top-level
// field, not as a message).
func translateMessages(messages []llm.Message) (string, []anthropicMessage, error) {
	var system string
	var out []anthropicMessage

	for _, m := range messages {
		switch m.Role {
		case llm.RoleSystem:
			if system != "" {
				system += "\n"
			}
			system += m.Content

		case llm.RoleUser:
			out = append(out, anthropicMessage{
				Role:    "user",
				Content: []anthropicContent{{Type: "text", Text: m.Content}},
			})

		case llm.RoleAssistant:
			var blocks []anthropicContent
			if m.Content != "" {
				blocks = append(blocks, anthropicContent{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				raw := json.RawMessage(tc.Arguments)
				if len(raw) == 0 {
					raw = json.RawMessage("{}")
				}
				blocks = append(blocks, anthropicContent{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: raw,
				})
			}
			if len(blocks) == 0 {
				continue
			}
			out = append(out, anthropicMessage{Role: "assistant", Content: blocks})

		case llm.RoleTool:
			toolBlock := anthropicContent{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   m.Content,
			}
			if len(out) > 0 && out[len(out)-1].Role == "user" {
				out[len(out)-1].Content = append(out[len(out)-1].Content, toolBlock)
			} else {
				out = append(out, anthropicMessage{
					Role:    "user",
					Content: []anthropicContent{toolBlock},
				})
			}

		default:
			return "", nil, fmt.Errorf("unsupported message role: %q", m.Role)
		}
	}

	return system, out, nil
}

func translateTools(tools []llm.Tool) []anthropicTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]anthropicTool, len(tools))
	for i, t := range tools {
		out[i] = anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		}
	}
	return out
}
