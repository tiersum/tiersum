// Package client defines interfaces for third-party systems (e.g. LLM HTTP APIs).
// Implementations live in subpackages such as client/llm. Cold chapter and query text vectors use coldindex.IColdTextEmbedder (see internal/storage/coldindex).
package client

import "context"

// LLMMessageRole defines supported roles for LLM chat messages.
type LLMMessageRole string

const (
	// LLMMessageRoleSystem is the system/instruction role.
	LLMMessageRoleSystem LLMMessageRole = "system"
	// LLMMessageRoleUser is the user query role.
	LLMMessageRoleUser LLMMessageRole = "user"
	// LLMMessageRoleAssistant is the assistant reply role (used for multi-turn history).
	LLMMessageRoleAssistant LLMMessageRole = "assistant"
)

// LLMMessage represents a single message in a chat completion request.
type LLMMessage struct {
	Role    LLMMessageRole
	Content string
}

// ILLMProvider defines LLM service interface
type ILLMProvider interface {
	// Generate generates text completion from a list of messages.
	// The implementation may prepend a system message (e.g. from config) if the first message is not system.
	Generate(ctx context.Context, messages []LLMMessage, maxTokens int) (string, error)
}
