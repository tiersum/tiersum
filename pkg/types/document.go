package types

import (
	"time"
)

// Document represents a document in the system
type Document struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Format    string    `json:"format"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SummaryTier represents the level of summarization
// Hierarchy: Document > Chapter > Source
// Note: Topic and Paragraph tiers are removed in the new architecture
type SummaryTier string

const (
	TierDocument SummaryTier = "document" // Document level summary
	TierChapter  SummaryTier = "chapter"  // Chapter/section level summary
	TierSource   SummaryTier = "source"   // Original source text
)

// Summary represents a summary at a specific tier
type Summary struct {
	ID         string      `json:"id"`
	DocumentID string      `json:"document_id"`
	Tier       SummaryTier `json:"tier"`
	Path       string      `json:"path"`    // Format: doc_id or doc_id/chapter_title
	Content    string      `json:"content"` // Summary content or source content
	IsSource   bool        `json:"is_source"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

// ChapterInfo represents a chapter/section in a document
type ChapterInfo struct {
	Title       string `json:"title"`       // Chapter title
	Level       int    `json:"level"`       // Header level (1=#, 2=##, 3=###)
	Summary     string `json:"summary"`     // Chapter summary
	Content     string `json:"content"`     // Chapter original content
	StartOffset int    `json:"start_offset"` // Start position in document
	EndOffset   int    `json:"end_offset"`   // End position in document
}

// DocumentAnalysisResult holds LLM analysis results for a document
type DocumentAnalysisResult struct {
	Summary  string        `json:"summary"`   // Document-level summary
	Tags     []string      `json:"tags"`      // Generated tags (max 10)
	Chapters []ChapterInfo `json:"chapters"`  // List of chapters with summaries
}

// TagGroup represents a cluster of tags (Level 1 categorization)
type TagGroup struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`        // Cluster name/category
	Description string   `json:"description"` // Cluster description
	Tags        []string `json:"tags"`        // Tags in this cluster
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Tag represents a global tag with its metadata
type Tag struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`        // Tag name
	ClusterID   string    `json:"cluster_id"`  // Which cluster it belongs to (Level 1)
	DocumentCount int     `json:"document_count"` // Number of documents with this tag
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateDocumentRequest represents a request to create a document
type CreateDocumentRequest struct {
	Title   string   `json:"title" binding:"required"`
	Content string   `json:"content" binding:"required"`
	Format  string   `json:"format" binding:"required,oneof=markdown md"`
	Tags    []string `json:"tags,omitempty"`
}

// CreateDocumentResponse represents the response from creating a document
type CreateDocumentResponse struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Format    string    `json:"format"`
	Tags      []string  `json:"tags"`
	Summary   string    `json:"summary"`
	ChapterCount int    `json:"chapter_count"`
	CreatedAt time.Time `json:"created_at"`
}

// QueryRequest represents a query request
type QueryRequest struct {
	Question string      `form:"question" binding:"required"`
	Depth    SummaryTier `form:"depth" binding:"omitempty,oneof=document chapter source"`
	Tags     []string    `form:"tags,omitempty"` // Optional tag filter
}

// QueryResponse represents a query response
type QueryResponse struct {
	Question string        `json:"question"`
	Depth    SummaryTier   `json:"depth"`
	Results  []QueryResult `json:"results"`
}

// QueryResult represents a single query result
type QueryResult struct {
	DocumentID    string      `json:"document_id"`
	DocumentTitle string      `json:"document_title"`
	Tier          SummaryTier `json:"tier"`
	Path          string      `json:"path"`
	Content       string      `json:"content"`
	Relevance     float64     `json:"relevance"`
}

// TagLevel represents the level of a tag in the two-level hierarchy
type TagLevel int

const (
	TagLevel1 TagLevel = 1 // Level 1: Cluster/Category
	TagLevel2 TagLevel = 2 // Level 2: Actual tag
)

// TagFilterResult represents a tag filter result from LLM
type TagFilterResult struct {
	Tag       string  `json:"tag"`
	Relevance float64 `json:"relevance"`
}
