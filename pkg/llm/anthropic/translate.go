package anthropic

import (
	"encoding/json"
	"fmt"

	"security-analyzer/pkg/llm"
)

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

// translateTools converts llm.Tool definitions into Anthropic tool declarations.
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
