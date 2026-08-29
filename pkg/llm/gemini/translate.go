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
			system = appendSystemText(system, m.Content)

		case llm.RoleUser:
			out = append(out, translateUserContent(m.Content))

		case llm.RoleAssistant:
			if content, ok := translateModelContent(m.Content, m.ToolCalls); ok {
				out = append(out, content)
			}

		case llm.RoleTool:
			out = appendToolResponseContent(out, m.Name, m.ToolCallID, m.Content)

		default:
			return "", nil, fmt.Errorf("unsupported message role: %q", m.Role)
		}
	}

	return system, out, nil
}

func appendSystemText(existing, addition string) string {
	if existing != "" {
		return existing + "\n" + addition
	}
	return addition
}

func translateUserContent(content string) geminiContent {
	return geminiContent{
		Role: "user",
		Parts: []geminiPart{
			{Text: content},
		},
	}
}

func translateModelContent(content string, toolCalls []llm.ToolCall) (geminiContent, bool) {
	var parts []geminiPart
	if content != "" {
		parts = append(parts, geminiPart{Text: content})
	}
	for _, tc := range toolCalls {
		parts = append(parts, translateFunctionCallPart(tc))
	}
	if len(parts) == 0 {
		return geminiContent{}, false
	}
	return geminiContent{Role: "model", Parts: parts}, true
}

func translateFunctionCallPart(tc llm.ToolCall) geminiPart {
	raw := json.RawMessage(tc.Arguments)
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	return geminiPart{
		FunctionCall: &geminiFunctionCall{
			Name: tc.Name,
			Args: raw,
		},
	}
}

func appendToolResponseContent(out []geminiContent, name, toolCallID, content string) []geminiContent {
	if name == "" {
		name = toolCallID
	}
	toolPart := geminiPart{
		FunctionResponse: &geminiFunctionResponse{
			Name:     name,
			Response: map[string]interface{}{"content": content},
		},
	}

	// Coalesce into preceding user turn if present, respecting Gemini turn alternation.
	if len(out) > 0 && out[len(out)-1].Role == "user" {
		lastIdx := len(out) - 1
		out[lastIdx].Parts = append(out[lastIdx].Parts, toolPart)
		return out
	}

	return append(out, geminiContent{
		Role:  "user",
		Parts: []geminiPart{toolPart},
	})
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
