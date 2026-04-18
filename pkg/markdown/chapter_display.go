// Package markdown holds neutral markdown helpers shared across layers (no internal/* imports).
package markdown

import "strings"

// ChapterDisplayTitle builds a human-readable section title from a stable chapter path under docID
// (e.g. "doc/Part/A" → "Part · A"). Used for API JSON and cold-ingest-aligned markdown extraction.
func ChapterDisplayTitle(docID, path, fallbackTitle string) string {
	rel := strings.TrimPrefix(path, docID+"/")
	if rel == "" {
		if strings.TrimSpace(fallbackTitle) != "" {
			return fallbackTitle
		}
		return "Document"
	}
	return strings.ReplaceAll(rel, "/", " · ")
}
