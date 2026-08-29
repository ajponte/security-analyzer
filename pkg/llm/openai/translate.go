package openai

import (
	"security-analyzer/pkg/llm"

	"github.com/sashabaranov/go-openai"
)

func translateMessages(messages []llm.Message) []openai.ChatCompletionMessage {
	reqMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		reqMessages[i] = translateMessage(m)
	}
	return reqMessages
}

func translateMessage(m llm.Message) openai.ChatCompletionMessage {
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

	return openai.ChatCompletionMessage{
		Role:       string(m.Role),
		Content:    m.Content,
		Name:       m.Name,
		ToolCalls:  toolCalls,
		ToolCallID: m.ToolCallID,
	}
}

func translateTools(tools []llm.Tool) []openai.Tool {
	if len(tools) == 0 {
		return nil
	}
	reqTools := make([]openai.Tool, len(tools))
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
	return reqTools
}

func translateResponseToolCalls(toolCalls []openai.ToolCall) []llm.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}
	resToolCalls := make([]llm.ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		resToolCalls[i] = llm.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		}
	}
	return resToolCalls
}
