package svcimpl

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/pkg/types"
)

const (
	progressiveAnswerMaxExcerptBytes = 6000
	progressiveAnswerMaxRefs         = 30
)

func (s *QuerySvc) generateProgressiveAnswer(ctx context.Context, question string, items []types.QueryItem) string {
	if s.llm == nil || len(items) == 0 {
		return ""
	}
	prompt := buildProgressiveAnswerPrompt(question, items)
	maxTok := progressiveAnswerMaxTokens()
	ans, err := s.llm.Generate(ctx, prompt, maxTok)
	if err != nil {
		s.logger.Warn("progressive query: answer generation failed", zap.Error(err))
		return ""
	}
	return strings.TrimSpace(ans)
}

func buildProgressiveAnswerPrompt(question string, items []types.QueryItem) string {
	n := len(items)
	if n > progressiveAnswerMaxRefs {
		n = progressiveAnswerMaxRefs
	}
	var b strings.Builder
	b.WriteString("Answer the user's question using ONLY the numbered reference excerpts below. ")
	b.WriteString("If the excerpts do not contain enough information, say so briefly. ")
	b.WriteString("Write in the same language as the user's question when possible. ")
	b.WriteString("Use Markdown. When citing a reference, use the marker [^N^] where N is the reference index (1-based), e.g. [^1^].\n\n")
	b.WriteString("--- References ---\n")
	for i := 0; i < n; i++ {
		it := items[i]
		excerpt := truncateUTF8ForPrompt(it.Content, progressiveAnswerMaxExcerptBytes)
		fmt.Fprintf(&b, "\n### Reference [^%d^]\n", i+1)
		fmt.Fprintf(&b, "Title: %s\n", it.Title)
		fmt.Fprintf(&b, "Document ID: %s\n", it.ID)
		fmt.Fprintf(&b, "Path: %s\n", it.Path)
		if it.Status != "" {
			fmt.Fprintf(&b, "Document status: %s\n", it.Status)
		}
		fmt.Fprintf(&b, "Tier: %s\n", string(it.Tier))
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

func progressiveAnswerMaxTokens() int {
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
