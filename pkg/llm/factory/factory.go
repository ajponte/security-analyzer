package factory

import (
	"fmt"
	"strings"

	"security-analyzer/pkg/config"
	"security-analyzer/pkg/llm"
	"security-analyzer/pkg/llm/anthropic"
	"security-analyzer/pkg/llm/gemini"
	"security-analyzer/pkg/llm/openai"
)

// NewClient creates an llm.LLMClient instance based on the provided LLM configuration.
func NewClient(cfg *config.LLMConfig) (llm.LLMClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("llm configuration cannot be nil")
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "openai"
	}

	switch provider {
	case "openai":
		return openai.NewClientWithConfig(cfg.OpenAIKey, cfg.Model)
	case "anthropic":
		return anthropic.NewClientWithConfig(cfg.AnthropicKey, cfg.Model)
	case "gemini":
		return gemini.NewClientWithConfig(cfg.GeminiKey, cfg.Model)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %q", cfg.Provider)
	}
}
