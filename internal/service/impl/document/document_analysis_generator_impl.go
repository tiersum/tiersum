package document

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/pkg/types"
)

// llmInputMaxRunes limits the content runes sent to the LLM.
// Output is separately capped by the maxTokens argument to Generate.
const llmInputMaxRunes = 50000

// NewDocumentAnalysisGenerator constructs the service.IDocumentAnalysisGenerator implementation.
func NewDocumentAnalysisGenerator(provider client.ILLMProvider, logger *zap.Logger) service.IDocumentAnalysisGenerator {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &documentAnalyzer{provider: provider, logger: logger}
}

type documentAnalyzer struct {
	provider client.ILLMProvider
	logger   *zap.Logger
}

// GenerateAnalysis runs a single LLM call, parses JSON into summary/tags/chapters.
// Chapter bodies are returned directly by the LLM in the "content" field.
// If the content exceeds the LLM input limit, it returns an error so callers can
// fall back to cold-index handling.
func (a *documentAnalyzer) GenerateAnalysis(ctx context.Context, title string, content string) (*types.DocumentAnalysisResult, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("llm provider not configured")
	}
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)

	runeCount := utf8.RuneCountInString(content)
	if runeCount > llmInputMaxRunes {
		return nil, fmt.Errorf("content length %d exceeds LLM input limit %d", runeCount, llmInputMaxRunes)
	}

	// Dynamic output budget: roughly 1 token per 2 runes plus headroom, bounded between 2k and 8k.
	maxTokens := runeCount/2 + 1000
	if maxTokens < 2000 {
		maxTokens = 2000
	}
	if maxTokens > 8000 {
		maxTokens = 8000
	}

	prompt := fmt.Sprintf(`Analyze the following document and provide a JSON response.

Title: %s

Full Content:
%s

Please analyze this document and return a JSON object with the following structure:
{
  "summary": "document summary (max 300 chars)",
  "tags": ["tag1", "tag2", ...],
  "chapters": [
    {
      "title": "chapter title",
      "summary": "chapter summary (max 150 chars, may be empty)",
      "content": "chapter original content (verbatim or trimmed)"
    }
  ]
}

Guidelines:
- Return ONLY the JSON object, no other text.
- Do NOT wrap the JSON in markdown code fences.
- Tags should be relevant keywords (lowercase, no spaces use-hyphens).
- Include at most 12 objects in "chapters".
- For EVERY chapter object you MUST include "title", "summary", and "content".
- "summary" may be empty when no short summary is appropriate; otherwise keep it within 150 characters.
- "content" should contain the chapter's original text. You may lightly trim trailing whitespace but preserve meaning.
- If the document has no clear chapters, create a single chapter spanning the full content (summary may be empty).
`, title, content)

	out, err := a.provider.Generate(ctx, prompt, maxTokens)
	if err != nil {
		a.logger.Warn("AnalyzeDocument: llm generate failed", zap.Error(err))
		return nil, err
	}
	out = strings.TrimSpace(out)
	res, perr := parseAnalysisJSON(out)
	if perr != nil {
		a.logger.Warn("AnalyzeDocument: parse failed", zap.Error(perr))
		return nil, fmt.Errorf("analyze document: parse json: %w", perr)
	}
	return res, nil
}

func parseAnalysisJSON(raw string) (*types.DocumentAnalysisResult, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty response")
	}
	raw = stripMarkdownCodeFence(raw)
	if obj, ok := extractFirstJSONObject(raw); ok {
		raw = obj
	}
	var res types.DocumentAnalysisResult
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		return nil, err
	}
	return &res, nil
}

var (
	// Matches a single fenced code block. This is intentionally conservative: we only use it as a fast-path to unwrap
	// fenced JSON responses. If the model produced extra text around the fence, extractFirstJSONObject handles it.
	codeFenceRe = regexp.MustCompile("(?s)^\\s*```[a-zA-Z0-9_-]*\\s*\\n(.*?)\\n```\\s*$")
)

func stripMarkdownCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	m := codeFenceRe.FindStringSubmatch(s)
	if len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return s
}

// extractFirstJSONObject returns the first balanced JSON object found in s. It is resilient to:
// - extra prose before/after the JSON
// - markdown formatting (handled by stripMarkdownCodeFence)
// - braces within JSON strings
func extractFirstJSONObject(s string) (string, bool) {
	start := strings.Index(s, "{")
	if start < 0 {
		return "", false
	}
	inStr := false
	escape := false
	depth := 0
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.TrimSpace(s[start : i+1]), true
			}
		}
	}
	return "", false
}

func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func titleOrDefault(t string) string {
	if strings.TrimSpace(t) == "" {
		return "Document"
	}
	return strings.TrimSpace(t)
}

var _ service.IDocumentAnalysisGenerator = (*documentAnalyzer)(nil)
