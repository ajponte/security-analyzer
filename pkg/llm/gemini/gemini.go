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

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client implements llm.LLMClient for Google Gemini models.
type Client struct {
	apiKey       string
	model        string
	endpointBase string
	http         httpDoer
}

// NewGeminiClient constructs a Client from environment configuration.
func NewGeminiClient() (*Client, error) {
	return NewGeminiClientWithConfig(os.Getenv("GEMINI_API_KEY"), os.Getenv("LLM_MODEL"))
}

// NewGeminiClientWithConfig constructs a Client with explicit API key and model.
func NewGeminiClientWithConfig(apiKey, model string) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("missing API key for Gemini")
	}

	if model == "" {
		model = defaultModel
	}

	return &Client{
		apiKey:       apiKey,
		model:        model,
		endpointBase: defaultEndpointBase,
		http:         http.DefaultClient,
	}, nil
}

// SetHTTPClient overrides the HTTP transport. Intended for tests.
func (c *Client) SetHTTPClient(doer httpDoer) {
	if doer != nil {
		c.http = doer
	}
}

// SetEndpointBase overrides the API base URL. Intended for tests.
func (c *Client) SetEndpointBase(base string) {
	if base != "" {
		c.endpointBase = base
	}
}

// --- Gemini wire format ---

type geminiRequest struct {
	Contents          []geminiContent         `json:"contents"`
	Tools             []geminiTool            `json:"tools,omitempty"`
	SystemInstruction *geminiContent          `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiGenerationConfig struct{}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type geminiFunctionResponse struct {
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations"`
}

type geminiFunctionDeclaration struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
	Error      *geminiError      `json:"error,omitempty"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}

type geminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
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

// translateMessages converts llm.Message entries into Gemini's format. The
// system prompt is returned separately (Gemini uses systemInstruction, not a
// role in the contents array).
func translateMessages(messages []llm.Message) (string, []geminiContent, error) {
	var system string
	var out []geminiContent

	for _, m := range messages {
		switch m.Role {
		case llm.RoleSystem:
			if system != "" {
				system += "\n"
			}
			system += m.Content

		case llm.RoleUser:
			out = append(out, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: m.Content}},
			})

		case llm.RoleAssistant:
			var parts []geminiPart
			if m.Content != "" {
				parts = append(parts, geminiPart{Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				raw := json.RawMessage(tc.Arguments)
				if len(raw) == 0 {
					raw = json.RawMessage("{}")
				}
				parts = append(parts, geminiPart{
					FunctionCall: &geminiFunctionCall{
						Name: tc.Name,
						Args: raw,
					},
				})
			}
			if len(parts) == 0 {
				continue
			}
			out = append(out, geminiContent{Role: "model", Parts: parts})

		case llm.RoleTool:
			name := m.Name
			if name == "" {
				name = m.ToolCallID
			}
			toolPart := geminiPart{
				FunctionResponse: &geminiFunctionResponse{
					Name:     name,
					Response: map[string]interface{}{"content": m.Content},
				},
			}
			if len(out) > 0 && out[len(out)-1].Role == "user" {
				out[len(out)-1].Parts = append(out[len(out)-1].Parts, toolPart)
			} else {
				out = append(out, geminiContent{
					Role:  "user",
					Parts: []geminiPart{toolPart},
				})
			}

		default:
			return "", nil, fmt.Errorf("unsupported message role: %q", m.Role)
		}
	}

	return system, out, nil
}

func translateTools(tools []llm.Tool) []geminiTool {
	if len(tools) == 0 {
		return nil
	}
	decls := make([]geminiFunctionDeclaration, len(tools))
	for i, t := range tools {
		decls[i] = geminiFunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		}
	}
	return []geminiTool{{FunctionDeclarations: decls}}
}
