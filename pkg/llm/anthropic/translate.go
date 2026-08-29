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
			system = appendSystemText(system, m.Content)

		case llm.RoleUser:
			out = append(out, translateUserMessage(m.Content))

		case llm.RoleAssistant:
			if msg, ok := translateAssistantMessage(m.Content, m.ToolCalls); ok {
				out = append(out, msg)
			}

		case llm.RoleTool:
			out = appendToolResultMessage(out, m.ToolCallID, m.Content)

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

func translateUserMessage(content string) anthropicMessage {
	return anthropicMessage{
		Role: "user",
		Content: []anthropicContent{
			{Type: "text", Text: content},
		},
	}
}

func translateAssistantMessage(content string, toolCalls []llm.ToolCall) (anthropicMessage, bool) {
	var blocks []anthropicContent
	if content != "" {
		blocks = append(blocks, anthropicContent{Type: "text", Text: content})
	}
	for _, tc := range toolCalls {
		blocks = append(blocks, translateToolCall(tc))
	}
	if len(blocks) == 0 {
		return anthropicMessage{}, false
	}
	return anthropicMessage{Role: "assistant", Content: blocks}, true
}

func translateToolCall(tc llm.ToolCall) anthropicContent {
	raw := json.RawMessage(tc.Arguments)
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	return anthropicContent{
		Type:  "tool_use",
		ID:    tc.ID,
		Name:  tc.Name,
		Input: raw,
	}
}

func appendToolResultMessage(out []anthropicMessage, toolUseID, content string) []anthropicMessage {
	toolBlock := anthropicContent{
		Type:      "tool_result",
		ToolUseID: toolUseID,
		Content:   content,
	}

	// Coalesce into preceding user turn if present, respecting Anthropic role alternation.
	if len(out) > 0 && out[len(out)-1].Role == "user" {
		lastIdx := len(out) - 1
		out[lastIdx].Content = append(out[lastIdx].Content, toolBlock)
		return out
	}

	return append(out, anthropicMessage{
		Role:    "user",
		Content: []anthropicContent{toolBlock},
	})
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
