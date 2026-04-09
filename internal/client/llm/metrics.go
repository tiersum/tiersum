// Package llm implements LLM provider with metrics tracking
package llm

import (
	"context"
	"strings"

	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/metrics"
)

// MetricTrackedProvider wraps an LLM provider to track metrics
type MetricTrackedProvider struct {
	provider client.ILLMProvider
	path     string
}

// NewMetricTrackedProvider creates a new metrics-tracking LLM provider
func NewMetricTrackedProvider(provider client.ILLMProvider, path string) *MetricTrackedProvider {
	return &MetricTrackedProvider{
		provider: provider,
		path:     path,
	}
}

// Generate implements ILLMProvider.Generate with metrics tracking
func (p *MetricTrackedProvider) Generate(ctx context.Context, prompt string, maxTokens int) (string, error) {
	// Estimate input tokens (rough approximation: 1 token ~ 4 chars for English, ~1.5 chars for Chinese)
	inputTokens := estimateTokens(prompt)

	// Record the call
	metrics.RecordLLMCall(p.path, inputTokens)

	// Perform actual LLM call
	return p.provider.Generate(ctx, prompt, maxTokens)
}

// estimateTokens provides a rough token estimate
// Uses character count divided by 2.5 as a heuristic for mixed content
func estimateTokens(text string) int {
	if text == "" {
		return 0
	}

	// Rough heuristic: 1 token ~ 2.5 characters for mixed content
	charCount := len(text)

	// Count Chinese characters (typically 1 char = 1 token)
	chineseCount := 0
	for _, r := range text {
		if r > 127 {
			chineseCount++
		}
	}

	// Adjust calculation based on Chinese character ratio
	if chineseCount > 0 {
		// Mixed content: Chinese chars + English tokens
		englishChars := charCount - chineseCount
		tokens := chineseCount + englishChars/4
		return tokens
	}

	// English only: ~4 chars per token
	return charCount / 4
}

// Helper function to classify prompt and determine path
func ClassifyPromptPath(prompt string) string {
	lower := strings.ToLower(prompt)

	switch {
	case strings.Contains(lower, "analyze") && strings.Contains(lower, "document"):
		return metrics.PathDocAnalyze
	case strings.Contains(lower, "l1") || strings.Contains(lower, "tag group"):
		return metrics.PathL1GroupFilter
	case strings.Contains(lower, "l2") || strings.Contains(lower, "tag") && strings.Contains(lower, "filter"):
		return metrics.PathL2TagFilter
	case strings.Contains(lower, "document") && strings.Contains(lower, "relevance"):
		return metrics.PathDocFilter
	case strings.Contains(lower, "chapter") || strings.Contains(lower, "section"):
		return metrics.PathChapterFilter
	case strings.Contains(lower, "group") && strings.Contains(lower, "tag"):
		return metrics.PathTagGroup
	default:
		return metrics.PathUnknown
	}
}

var _ client.ILLMProvider = (*MetricTrackedProvider)(nil)
