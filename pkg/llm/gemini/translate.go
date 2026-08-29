package gemini

import (
	"encoding/json"
	"fmt"

	"security-analyzer/pkg/llm"
)

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
