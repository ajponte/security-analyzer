package openai

import (
	"context"
	"errors"
	"net/http"
	"os"

	"security-analyzer/pkg/llm"

	"github.com/sashabaranov/go-openai"
)

const defaultModel = "gpt-4o-mini"

// Client implements llm.LLMClient using go-openai.
type Client struct {
	client *openai.Client
	config openai.ClientConfig
	model  string
}

// OpenAIClient is a type alias for Client to preserve backward compatibility.
type OpenAIClient = Client

// Option configures the OpenAI Client.
type Option func(*Client)

// WithHTTPClient configures a custom HTTP transport client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.config.HTTPClient = httpClient
			c.client = openai.NewClientWithConfig(c.config)
		}
	}
}

// WithBaseURL configures a custom API base URL.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		if baseURL != "" {
			c.config.BaseURL = baseURL
			c.client = openai.NewClientWithConfig(c.config)
		}
	}
}

// NewClient initializes the OpenAI client using environment variables.
func NewClient(opts ...Option) (*Client, error) {
	return NewClientWithConfig(os.Getenv("OPENAI_API_KEY"), os.Getenv("LLM_MODEL"), opts...)
}

// NewClientWithConfig initializes the OpenAI client with explicit API key, model, and functional options.
func NewClientWithConfig(apiKey, model string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("missing API key for OpenAI")
	}

	if model == "" {
		model = defaultModel
	}

	config := openai.DefaultConfig(apiKey)
	c := &Client{
		client: openai.NewClientWithConfig(config),
		config: config,
		model:  model,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// NewOpenAIClient is an alias for NewClient for backward compatibility.
func NewOpenAIClient(opts ...Option) (*Client, error) {
	return NewClient(opts...)
}

// NewOpenAIClientWithConfig is an alias for NewClientWithConfig for backward compatibility.
func NewOpenAIClientWithConfig(apiKey, model string, opts ...Option) (*Client, error) {
	return NewClientWithConfig(apiKey, model, opts...)
}

// SetBaseURL overrides the API base URL (useful for testing against mock servers).
func (c *Client) SetBaseURL(baseURL string) {
	WithBaseURL(baseURL)(c)
}

// SetHTTPClient overrides the HTTP transport client.
func (c *Client) SetHTTPClient(httpClient *http.Client) {
	WithHTTPClient(httpClient)(c)
}

// GenerateResponse generates a response from the OpenAI chat model.
func (c *Client) GenerateResponse(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	reqMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		var toolCalls []openai.ToolCall
		if len(m.ToolCalls) > 0 {
			toolCalls = make([]openai.ToolCall, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				toolCalls[j] = openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				}
			}
		}

		reqMessages[i] = openai.ChatCompletionMessage{
			Role:       string(m.Role),
			Content:    m.Content,
			Name:       m.Name,
			ToolCalls:  toolCalls,
			ToolCallID: m.ToolCallID,
		}
	}

	var reqTools []openai.Tool
	if len(tools) > 0 {
		reqTools = make([]openai.Tool, len(tools))
		for i, t := range tools {
			reqTools[i] = openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Parameters,
				},
			}
		}
	}

	req := openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: reqMessages,
		Tools:    reqTools,
	}

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("openai returned empty choices")
	}

	choice := resp.Choices[0]

	var resToolCalls []llm.ToolCall
	if len(choice.Message.ToolCalls) > 0 {
		resToolCalls = make([]llm.ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			resToolCalls[i] = llm.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}
		}
	}

	return &llm.Response{
		Content:   choice.Message.Content,
		ToolCalls: resToolCalls,
	}, nil
}
