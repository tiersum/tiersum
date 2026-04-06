// Package client defines client layer interfaces
// Third-party system dependencies
package client

import "context"

// ILLMProvider defines LLM service interface
type ILLMProvider interface {
	// Generate generates text completion
	Generate(ctx context.Context, prompt string, maxTokens int) (string, error)
}
