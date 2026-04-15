package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/pkg/types"
)

const (
	// progressiveAnswerExcerptBytesCeiling caps per-reference excerpt size in the answer prompt (bytes).
	progressiveAnswerExcerptBytesCeiling = 6000
	// progressiveAnswerReferencesCeiling caps how many references are passed to the answer LLM.
	progressiveAnswerReferencesCeiling = 30

	defaultProgressiveAnswerMaxReferences = 12
	defaultProgressiveAnswerExcerptBytes  = 3500
	defaultProgressiveAnswerOutTokens     = 900
)

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// progressiveAnswerMaxReferences limits references in the answer prompt (smaller = less input latency).
func progressiveAnswerMaxReferences() int {
	v := viper.GetInt("query.progressive_answer_max_references")
	if v <= 0 {
		v = defaultProgressiveAnswerMaxReferences
	}
	return clampInt(v, 3, progressiveAnswerReferencesCeiling)
}

// progressiveAnswerExcerptMaxBytes limits bytes per excerpt in the answer prompt.
func progressiveAnswerExcerptMaxBytes() int {
	v := viper.GetInt("query.progressive_answer_excerpt_max_bytes")
	if v <= 0 {
		v = defaultProgressiveAnswerExcerptBytes
	}
	return clampInt(v, 800, progressiveAnswerExcerptBytesCeiling)
}

// progressiveAnswerCompletionMaxTokens is the completion budget for the synthesized answer only.
// Defaults to min(provider max_tokens, defaultProgressiveAnswerOutTokens) unless query.progressive_answer_max_tokens is set.
func progressiveAnswerCompletionMaxTokens() int {
	const minTok, maxTok = 64, 8192
	if v := viper.GetInt("query.progressive_answer_max_tokens"); v > 0 {
		return clampInt(v, minTok, maxTok)
	}
	global := progressiveAnswerProviderMaxTokens()
	if global <= 0 {
		global = 2000
	}
	if global < defaultProgressiveAnswerOutTokens {
		return global
	}
	return defaultProgressiveAnswerOutTokens
}

// progressiveAnswerProviderMaxTokens reads the configured LLM max_tokens for the active provider.
func progressiveAnswerProviderMaxTokens() int {
	p := strings.ToLower(viper.GetString("llm.provider"))
	var n int
	switch p {
	case "anthropic", "claude":
		n = viper.GetInt("llm.anthropic.max_tokens")
	default:
		n = viper.GetInt("llm.openai.max_tokens")
	}
	if n <= 0 {
		return 2000
	}
	return n
}

func (s *queryService) generateProgressiveAnswer(ctx context.Context, question string, items []types.QueryItem) string {
	if s.llm == nil || len(items) == 0 {
		return ""
	}
	prompt := buildProgressiveAnswerPrompt(question, items)
	maxTok := progressiveAnswerCompletionMaxTokens()
	ans, err := s.llm.Generate(ctx, prompt, maxTok)
	if err != nil {
		s.logger.Warn("progressive query: answer generation failed", zap.Error(err))
		return ""
	}
	return strings.TrimSpace(ans)
}

func buildProgressiveAnswerPrompt(question string, items []types.QueryItem) string {
	maxRefs := progressiveAnswerMaxReferences()
	n := len(items)
	if n > maxRefs {
		n = maxRefs
	}
	var b strings.Builder
	b.WriteString("Answer the user's question using ONLY the numbered reference excerpts below. ")
	b.WriteString("If the excerpts do not contain enough information, say so briefly. ")
	b.WriteString("Write in the same language as the user's question when possible.\n\n")
	b.WriteString("Output format (required): Your entire reply MUST be valid GitHub-Flavored Markdown suitable for direct rendering. ")
	b.WriteString("Use headings (##, ###), bullet or numbered lists, **bold**, and `inline code` where it improves clarity; use fenced code blocks only for actual code or verbatim snippets. ")
	b.WriteString("You may use Markdown tables when comparing items. Do not wrap the whole answer in a single outer code fence. ")
	b.WriteString("Do not output HTML unless it appears inside a fenced code block as an example.\n\n")
	b.WriteString("Citations: when you refer to a reference, include the marker [^N^] where N is the reference index (1-based), e.g. [^1^].\n\n")
	b.WriteString("--- References ---\n")
	for i := 0; i < n; i++ {
		it := items[i]
		excerpt := truncateUTF8ForPrompt(it.Content, progressiveAnswerExcerptMaxBytes())
		fmt.Fprintf(&b, "\n### Reference [^%d^]\n", i+1)
		fmt.Fprintf(&b, "Title: %s\n", it.Title)
		fmt.Fprintf(&b, "Document ID: %s\n", it.ID)
		fmt.Fprintf(&b, "Path: %s\n", it.Path)
		if it.Status != "" {
			fmt.Fprintf(&b, "Document status: %s\n", it.Status)
		}
		fmt.Fprintf(&b, "Relevance (0-1): %.4f\n", it.Relevance)
		b.WriteString("Excerpt:\n")
		b.WriteString(excerpt)
		b.WriteByte('\n')
	}
	b.WriteString("\n--- User question ---\n")
	b.WriteString(strings.TrimSpace(question))
	b.WriteByte('\n')
	return b.String()
}

func truncateUTF8ForPrompt(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	s = s[:maxBytes]
	for len(s) > 0 && s[len(s)-1]&0xc0 == 0x80 {
		s = s[:len(s)-1]
	}
	return s + "\n…(truncated)"
}
