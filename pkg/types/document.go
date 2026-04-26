package types

import (
	"strings"
	"time"
)

// DocumentStatus is the persisted lifecycle/status of a document in the hot/cold model (not a summaries-table tier).
type DocumentStatus string

const (
	// DocStatusHot indicates a frequently accessed document with full LLM analysis
	DocStatusHot DocumentStatus = "hot"
	// DocStatusCold indicates a rarely accessed document with minimal processing
	DocStatusCold DocumentStatus = "cold"
	// DocStatusWarming indicates a document being promoted from cold to hot
	DocStatusWarming DocumentStatus = "warming"
)

// Document represents a document in the system
type Document struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Summary is the document-level summary (persisted for hot/warming docs; often empty for cold docs).
	Summary     string         `json:"summary,omitempty"`
	Content     string         `json:"content"`
	Format      string         `json:"format"`
	Tags        []string       `json:"tags,omitempty"`
	Status      DocumentStatus `json:"status"`
	HotScore    float64        `json:"hot_score"`
	QueryCount  int            `json:"query_count"`
	LastQueryAt *time.Time     `json:"last_query_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// Chapter represents one persisted hot-document chapter row.
// Schema: chapters(id, document_id, path, title, summary, content, created_at, updated_at)
type Chapter struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	Path       string    `json:"path"`
	Title      string    `json:"title"`
	Summary    string    `json:"summary"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ChapterInfo represents a chapter/section in a document
type ChapterInfo struct {
	Title   string `json:"title"`   // Chapter title
	Level   int    `json:"level"`   // Header level (1=#, 2=##, 3=###)
	Summary string `json:"summary"` // Chapter summary
	Content string `json:"content"` // Chapter original content
}

// DocumentAnalysisResult holds LLM analysis results for a document
type DocumentAnalysisResult struct {
	Summary  string        `json:"summary"`  // Document-level summary
	Tags     []string      `json:"tags"`     // Generated tags (max 10)
	Chapters []ChapterInfo `json:"chapters"` // List of chapters with summaries
}

// Topic is a high-level theme (LLM cluster of catalog tags).
type Topic struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// TagNames are member tag names assigned to this topic by regrouping (LLM output uses JSON key "tags").
	TagNames  []string  `json:"tag_names"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Tag is one catalog tag (deduplicated across documents), optionally linked to a Topic.
type Tag struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	TopicID       string    `json:"topic_id"`
	DocumentCount int       `json:"document_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// DocumentIngestMode selects how the platform chooses hot vs cold on ingest.
const (
	DocumentIngestModeHot  = "hot"
	DocumentIngestModeCold = "cold"
)

// NormalizeDocumentIngestMode returns hot or cold (unknown values become hot).
func NormalizeDocumentIngestMode(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case DocumentIngestModeHot:
		return DocumentIngestModeHot
	case DocumentIngestModeCold:
		return DocumentIngestModeCold
	default:
		return DocumentIngestModeHot
	}
}

// HotIngestWork is queued after a hot document is persisted when LLM analysis
// and hierarchical indexing run asynchronously (see job.HotIngestQueue).
type HotIngestWork struct {
	DocID string
	// PrebuiltTags is non-empty when the client supplied tags that must be merged
	// with tags returned from AnalyzeDocument.
	PrebuiltTags []string
}

// CreateDocumentRequest represents a request to create a document
type CreateDocumentRequest struct {
	Title   string   `json:"title" binding:"required"`
	Content string   `json:"content" binding:"required"`
	Format  string   `json:"format" binding:"required,oneof=markdown md"`
	Tags    []string `json:"tags,omitempty"`
	// IngestMode: hot = LLM semantic chapter extraction & summary; cold = Markdown syntax chapter extraction.
	IngestMode string `json:"ingest_mode,omitempty" binding:"omitempty,oneof=hot cold HOT COLD"`
	// ForceHot is deprecated: use ingest_mode "hot". When ingest_mode is empty and ForceHot is true, behavior is hot.
	ForceHot bool `json:"force_hot,omitempty"`
	// Summary is pre-generated document summary (from external agent)
	Summary string `json:"summary,omitempty"`
	// Chapters are pre-generated chapter summaries (from external agent)
	Chapters []ChapterInfo `json:"chapters,omitempty"`
	// Embedding is pre-computed vector embedding (from external agent)
	Embedding []float32 `json:"embedding,omitempty"`
}

// EffectiveIngestMode resolves ingest tier: non-empty ingest_mode wins (case-insensitive); else legacy force_hot; else hot.
func (r CreateDocumentRequest) EffectiveIngestMode() string {
	if strings.TrimSpace(r.IngestMode) != "" {
		return NormalizeDocumentIngestMode(r.IngestMode)
	}
	if r.ForceHot {
		return DocumentIngestModeHot
	}
	return DocumentIngestModeHot
}

// DocumentStatusCounts aggregates document rows by hot/cold/warming status (full-table SQL aggregate).
type DocumentStatusCounts struct {
	Total   int `json:"total"`
	Hot     int `json:"hot"`
	Cold    int `json:"cold"`
	Warming int `json:"warming"`
}

// CreateDocumentResponse represents the response from creating a document
type CreateDocumentResponse struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Format       string         `json:"format"`
	Tags         []string       `json:"tags"`
	Summary      string         `json:"summary"`
	ChapterCount int            `json:"chapter_count"`
	Status       DocumentStatus `json:"status"`
	CreatedAt    time.Time      `json:"created_at"`
}

// TagFilterResult represents a tag filter result from LLM
type TagFilterResult struct {
	Tag       string  `json:"tag"`
	Relevance float64 `json:"relevance"`
}
