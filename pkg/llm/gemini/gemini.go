// Package gemini provides an llm.LLMClient implementation backed by Google's
// Gemini generateContent API.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"security-analyzer/pkg/llm"
)

const (
	defaultModel        = "gemini-2.5-flash"
	defaultEndpointBase = "https://generativelanguage.googleapis.com/v1beta/models"
)

// Option configures the Gemini Client.
type Option func(*Client)

// WithHTTPClient sets a custom HTTP client for the Gemini Client.
func WithHTTPClient(httpClient llm.HTTPClient) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.http = httpClient
		}
	}
}

// WithEndpointBase sets a custom API base URL for the Gemini Client.
func WithEndpointBase(base string) Option {
	return func(c *Client) {
		if base != "" {
			c.endpointBase = base
		}
	}
}

// Client implements llm.LLMClient for Google Gemini models.
type Client struct {
	apiKey       string
	model        string
	endpointBase string
	http         llm.HTTPClient
}

// GeminiClient is an alias for Client for backward compatibility.
type GeminiClient = Client

// NewClient constructs a Client from environment configuration.
func NewClient(opts ...Option) (*Client, error) {
	return NewClientWithConfig(os.Getenv("GEMINI_API_KEY"), os.Getenv("LLM_MODEL"), opts...)
}

// NewClientWithConfig constructs a Client with explicit API key, model, and functional options.
func NewClientWithConfig(apiKey, model string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("missing API key for Gemini")
	}

	if model == "" {
		model = defaultModel
	}

	c := &Client{
		apiKey:       apiKey,
		model:        model,
		endpointBase: defaultEndpointBase,
		http:         http.DefaultClient,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// NewGeminiClient is an alias for NewClient for backward compatibility.
func NewGeminiClient(opts ...Option) (*Client, error) {
	return NewClient(opts...)
}

// NewGeminiClientWithConfig is an alias for NewClientWithConfig for backward compatibility.
func NewGeminiClientWithConfig(apiKey, model string, opts ...Option) (*Client, error) {
	return NewClientWithConfig(apiKey, model, opts...)
}

// SetHTTPClient overrides the HTTP transport. Intended for tests.
func (c *Client) SetHTTPClient(httpClient llm.HTTPClient) {
	WithHTTPClient(httpClient)(c)
}

// SetEndpointBase overrides the API base URL. Intended for tests.
func (c *Client) SetEndpointBase(base string) {
	WithEndpointBase(base)(c)
}

// GenerateResponse implements llm.LLMClient.
func (c *Client) GenerateResponse(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	system, contents, err := translateMessages(messages)
	if err != nil {
		return nil, fmt.Errorf("translating messages: %w", err)
	}

	reqBody := geminiRequest{
		Contents: contents,
		Tools:    translateTools(tools),
	}
	if system != "" {
		reqBody.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: system}},
		}
	}

	buf, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/%s:generateContent?key=%s",
		c.endpointBase, url.PathEscape(c.model), url.QueryEscape(c.apiKey))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("content-type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gemini API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	var parsed geminiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("gemini API error: %s (%d): %s", parsed.Error.Status, parsed.Error.Code, parsed.Error.Message)
	}
	if len(parsed.Candidates) == 0 {
		return nil, errors.New("gemini returned no candidates")
	}

	out := &llm.Response{}
	callCounter := 0
	for _, part := range parsed.Candidates[0].Content.Parts {
		switch {
		case part.FunctionCall != nil:
			callCounter++
			args := string(part.FunctionCall.Args)
			if args == "" {
				args = "{}"
			}
			out.ToolCalls = append(out.ToolCalls, llm.ToolCall{
				ID:        fmt.Sprintf("call_%d", callCounter),
				Name:      part.FunctionCall.Name,
				Arguments: args,
			})
		case part.Text != "":
			if out.Content != "" {
				out.Content += "\n"
			}
			out.Content += part.Text
		}
	}
	return out, nil
}
