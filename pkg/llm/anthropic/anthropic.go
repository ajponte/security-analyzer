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

// Option configures the Anthropic Client.
type Option func(*Client)

// WithHTTPClient sets a custom HTTP client for the Anthropic Client.
func WithHTTPClient(httpClient llm.HTTPClient) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.http = httpClient
		}
	}
}

// WithEndpoint sets a custom endpoint for the Anthropic Client.
func WithEndpoint(endpoint string) Option {
	return func(c *Client) {
		if endpoint != "" {
			c.endpoint = endpoint
		}
	}
}

// WithMaxTokens sets max tokens for generated completions.
func WithMaxTokens(maxTokens int) Option {
	return func(c *Client) {
		if maxTokens > 0 {
			c.maxTokens = maxTokens
		}
	}
}

// Client implements llm.LLMClient for Anthropic Claude models.
type Client struct {
	apiKey    string
	model     string
	endpoint  string
	maxTokens int
	http      llm.HTTPClient
}

// AnthropicClient is an alias for Client for backward compatibility.
type AnthropicClient = Client

// NewClient constructs a Client from environment configuration.
func NewClient(opts ...Option) (*Client, error) {
	return NewClientWithConfig(os.Getenv("ANTHROPIC_API_KEY"), os.Getenv("LLM_MODEL"), opts...)
}

// NewClientWithConfig constructs a Client with explicit API key, model, and functional options.
func NewClientWithConfig(apiKey, model string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("missing API key for Anthropic")
	}

	if model == "" {
		model = defaultModel
	}

	c := &Client{
		apiKey:    apiKey,
		model:     model,
		endpoint:  defaultEndpoint,
		maxTokens: defaultMaxTokens,
		http:      http.DefaultClient,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// NewAnthropicClient is an alias for NewClient for backward compatibility.
func NewAnthropicClient(opts ...Option) (*Client, error) {
	return NewClient(opts...)
}

// NewAnthropicClientWithConfig is an alias for NewClientWithConfig for backward compatibility.
func NewAnthropicClientWithConfig(apiKey, model string, opts ...Option) (*Client, error) {
	return NewClientWithConfig(apiKey, model, opts...)
}

// SetHTTPClient overrides the HTTP transport used by the client. Intended for tests.
func (c *Client) SetHTTPClient(httpClient llm.HTTPClient) {
	WithHTTPClient(httpClient)(c)
}

// SetEndpoint overrides the Messages API endpoint. Intended for tests.
func (c *Client) SetEndpoint(endpoint string) {
	WithEndpoint(endpoint)(c)
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
