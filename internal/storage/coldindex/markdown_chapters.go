package coldindex

import (
	"strings"

	"github.com/tiersum/tiersum/pkg/markdown"
	"github.com/tiersum/tiersum/pkg/types"
)

// MarkdownChaptersFromSplit builds types.Chapter rows using the same splitter and token budget as cold ingest.
// Display titles come from pkg/markdown (neutral), not from index ranking or Bleve mappings.
func MarkdownChaptersFromSplit(splitter IColdChapterSplitter, maxTokens int, docID, title, markdownBody string) []types.Chapter {
	if maxTokens <= 0 {
		maxTokens = types.DefaultColdChapterMaxTokens
	}
	if splitter == nil {
		splitter = DefaultColdChapterSplitter()
	}
	parts := splitter.Split(docID, title, markdownBody, maxTokens)
	out := make([]types.Chapter, 0, len(parts))
	for _, p := range parts {
		text := strings.TrimSpace(p.Text)
		if text == "" {
			continue
		}
		out = append(out, types.Chapter{
			DocumentID: docID,
			Path:       p.Path,
			Title:      markdown.ChapterDisplayTitle(docID, p.Path, title),
			// Extracted chapters do not have a separate summary; keep Summary == Content for UI and filtering reuse.
			Summary: text,
			Content: text,
		})
	}
	if len(out) == 0 {
		md := strings.TrimSpace(markdownBody)
		if md == "" {
			return nil
		}
		p := docID + "/body"
		return []types.Chapter{{
			DocumentID: docID,
			Path:       p,
			Title:      markdown.ChapterDisplayTitle(docID, p, title),
			Summary:    md,
			Content:    md,
		}}
	}
	return out
}
