package factory

import (
	"testing"

	"security-analyzer/pkg/config"
)

func TestNewClient(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		_, err := NewClient(nil)
		if err == nil {
			t.Fatal("expected error for nil config")
		}
	})

	t.Run("unsupported provider", func(t *testing.T) {
		cfg := &config.LLMConfig{
			Provider: "unknown-provider",
		}
		_, err := NewClient(cfg)
		if err == nil {
			t.Fatal("expected error for unsupported provider")
		}
	})

	t.Run("openai provider missing key", func(t *testing.T) {
		cfg := &config.LLMConfig{
			Provider:  "openai",
			OpenAIKey: "",
		}
		_, err := NewClient(cfg)
		if err == nil {
			t.Fatal("expected error for missing openai key")
		}
	})

	t.Run("openai provider success", func(t *testing.T) {
		cfg := &config.LLMConfig{
			Provider:  "openai",
			OpenAIKey: "test-key",
			Model:     "gpt-4o",
		}
		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client == nil {
			t.Fatal("expected non-nil client")
		}
	})

	t.Run("anthropic provider success", func(t *testing.T) {
		cfg := &config.LLMConfig{
			Provider:     "anthropic",
			AnthropicKey: "test-key",
			Model:        "claude-3-5-sonnet-latest",
		}
		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client == nil {
			t.Fatal("expected non-nil client")
		}
	})

	t.Run("gemini provider success", func(t *testing.T) {
		cfg := &config.LLMConfig{
			Provider:  "gemini",
			GeminiKey: "test-key",
			Model:     "gemini-2.5-flash",
		}
		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client == nil {
			t.Fatal("expected non-nil client")
		}
	})
}
