package document

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/pkg/types"
)

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

// GenerateAnalysis asks the LLM to produce a JSON analysis (summary, tags, chapters).
// It is best-effort: on parse errors, it returns a conservative fallback.
func (a *documentAnalyzer) GenerateAnalysis(ctx context.Context, title string, content string) (*types.DocumentAnalysisResult, error) {
	if a.provider == nil {
		return fallbackAnalysis(title, content), fmt.Errorf("LLM provider not configured")
	}
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)

	chapters := extractMarkdownChapters(content)
	var chapterContext strings.Builder
	for i, ch := range chapters {
		fmt.Fprintf(&chapterContext, "\nChapter %d: %s\n", i+1, ch.Title)
		fmt.Fprintf(&chapterContext, "Content preview: %s\n", truncateString(ch.Content, 500))
	}

	prompt := fmt.Sprintf(`Analyze the following document and provide a JSON response.

Title: %s

Full Content:
%s

Chapters identified:%s

Please analyze this document and return a JSON object with the following structure:
{
  "summary": "document summary (max 300 chars)",
  "tags": ["tag1", "tag2", ...], // Up to 10 tags
  "chapters": [
    {
      "title": "chapter title",
      "summary": "chapter summary (max 200 chars)",
      "content": "full chapter content"
    }
  ]
}

Guidelines:
- Return ONLY the JSON object, no other text.
- Tags should be relevant keywords (lowercase, no spaces use-hyphens).
- For EVERY chapter object you MUST include all three fields "title", "summary", and "content".
- "summary" is REQUIRED and must be NON-EMPTY: write 2-4 sentences capturing that chapter only.
- If the document has no clear chapters, create a single chapter with the full content and a non-empty summary.
`, title, truncateString(content, 10000), chapterContext.String())

	out, err := a.provider.Generate(ctx, prompt, 4000)
	if err != nil {
		a.logger.Warn("AnalyzeDocument: llm generate failed", zap.Error(err))
		return fallbackAnalysis(title, content), err
	}
	res, perr := parseAnalysisJSON(out)
	if perr != nil {
		a.logger.Warn("AnalyzeDocument: parse failed, using fallback", zap.Error(perr))
		return fallbackAnalysis(title, content), nil
	}
	ensureChapterSummaries(res)
	res.Tags = normalizeTags(res.Tags, 10)
	if len(res.Chapters) == 0 {
		res.Chapters = []types.ChapterInfo{{
			Title:   titleOrDefault(title),
			Summary: truncateString(res.Summary, 200),
			Content: content,
		}}
	}
	return res, nil
}

func titleOrDefault(t string) string {
	if strings.TrimSpace(t) == "" {
		return "Document"
	}
	return strings.TrimSpace(t)
}

func normalizeTags(tags []string, max int) []string {
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		t = strings.ReplaceAll(t, " ", "-")
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
		if max > 0 && len(out) >= max {
			break
		}
	}
	return out
}

func ensureChapterSummaries(res *types.DocumentAnalysisResult) {
	if res == nil {
		return
	}
	for i := range res.Chapters {
		ch := &res.Chapters[i]
		ch.Title = strings.TrimSpace(ch.Title)
		ch.Summary = strings.TrimSpace(ch.Summary)
		if ch.Summary == "" {
			// Defensive: never allow empty summaries (prompts require non-empty).
			ch.Summary = truncateString(strings.TrimSpace(ch.Content), 200)
			if ch.Summary == "" {
				ch.Summary = "Section summary unavailable."
			}
		}
	}
}

func parseAnalysisJSON(raw string) (*types.DocumentAnalysisResult, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty response")
	}
	// Try to extract the first JSON object when the model wrapped it.
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			raw = raw[i : j+1]
		}
	}
	var res types.DocumentAnalysisResult
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		return nil, err
	}
	res.Summary = strings.TrimSpace(res.Summary)
	for i := range res.Tags {
		res.Tags[i] = strings.TrimSpace(res.Tags[i])
	}
	return &res, nil
}

func fallbackAnalysis(title, content string) *types.DocumentAnalysisResult {
	t := titleOrDefault(title)
	body := strings.TrimSpace(content)
	return &types.DocumentAnalysisResult{
		Summary: truncateString(body, 300),
		Tags:    []string{},
		Chapters: []types.ChapterInfo{{
			Title:   t,
			Summary: truncateString(body, 200),
			Content: body,
		}},
	}
}

type mdChapter struct {
	Title   string
	Content string
}

var mdHeadingRe = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+?)\s*$`)

func extractMarkdownChapters(md string) []mdChapter {
	md = strings.TrimSpace(md)
	if md == "" {
		return []mdChapter{{Title: "Document", Content: ""}}
	}
	matches := mdHeadingRe.FindAllStringSubmatchIndex(md, -1)
	if len(matches) == 0 {
		return []mdChapter{{Title: "Document", Content: md}}
	}
	out := make([]mdChapter, 0, len(matches))
	for i, m := range matches {
		// m[0]:start of whole, m[1]:end of whole; m[4]:start of title, m[5]:end of title
		title := strings.TrimSpace(md[m[4]:m[5]])
		start := m[1]
		end := len(md)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		body := strings.TrimSpace(md[start:end])
		out = append(out, mdChapter{Title: title, Content: body})
	}
	return out
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

var _ service.IDocumentAnalysisGenerator = (*documentAnalyzer)(nil)

