// Package client defines interfaces for third-party systems (e.g. LLM HTTP APIs).
// Implementations live in subpackages such as client/llm. Cold chapter and query text vectors use coldindex.IColdTextEmbedder (see internal/storage/coldindex).
package client

import "context"

// ILLMProvider defines LLM service interface
type ILLMProvider interface {
	// Generate generates text completion
	Generate(ctx context.Context, prompt string, maxTokens int) (string, error)
}
