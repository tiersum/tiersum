// Package llm implements third-party client layer
package llm

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/client"
)

// ProviderFactory creates LLM providers based on configuration
type ProviderFactory struct {
	logger *zap.Logger
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory(logger *zap.Logger) *ProviderFactory {
	return &ProviderFactory{logger: logger}
}

// CreateProvider creates the appropriate LLM provider based on configuration
func (f *ProviderFactory) CreateProvider() (client.ILLMProvider, error) {
	providerName := strings.ToLower(viper.GetString("llm.provider"))

	if providerName == "" {
		providerName = "openai"
	}

	f.logger.Info("creating LLM provider", zap.String("provider", providerName))

	switch providerName {
	case "openai":
		return f.createOpenAIProvider()
	case "anthropic", "claude":
		return f.createAnthropicProvider()
	case "local", "ollama":
		return f.createOllamaProvider()
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s (supported: openai, anthropic, local/ollama)", providerName)
	}
}

func (f *ProviderFactory) createOpenAIProvider() (client.ILLMProvider, error) {
	apiKey := viper.GetString("llm.openai.api_key")
	if apiKey == "" {
		return nil, fmt.Errorf("llm.openai.api_key is required when using openai provider (check if environment variable is set)")
	}

	baseURL := viper.GetString("llm.openai.base_url")
	model := viper.GetString("llm.openai.model")
	
	// Debug: log API key prefix
	keyPrefix := apiKey
	if len(apiKey) > 15 {
		keyPrefix = apiKey[:15] + "..."
	}
	f.logger.Info("configured OpenAI provider", 
		zap.String("api_key_prefix", keyPrefix),
		zap.String("base_url", baseURL), 
		zap.String("model", model))

	provider := NewOpenAIProvider()
	return provider, nil
}

func (f *ProviderFactory) createAnthropicProvider() (client.ILLMProvider, error) {
	apiKey := viper.GetString("llm.anthropic.api_key")
	if apiKey == "" {
		return nil, fmt.Errorf("llm.anthropic.api_key is required when using anthropic provider (check if environment variable is set)")
	}

	provider := NewAnthropicProvider()
	return provider, nil
}

func (f *ProviderFactory) createOllamaProvider() (client.ILLMProvider, error) {
	baseURL := viper.GetString("llm.local.base_url")
	if baseURL == "" {
		return nil, fmt.Errorf("llm.local.base_url is required when using local/ollama provider")
	}

	provider := NewOllamaProvider()
	return provider, nil
}
