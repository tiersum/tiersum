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
// Hierarchy: Topic > Document > Chapter > Paragraph > Source
type SummaryTier string

const (
	TierTopic     SummaryTier = "topic"     // Highest level - theme/topic summary across documents
	TierDocument  SummaryTier = "document"  // Document level summary
	TierChapter   SummaryTier = "chapter"   // Chapter/section level
	TierParagraph SummaryTier = "paragraph" // Paragraph level
	TierSource    SummaryTier = "source"    // Original source text
)

// Summary represents a summary at a specific tier
type Summary struct {
	ID         string      `json:"id"`
	DocumentID string      `json:"document_id"`
	Tier       SummaryTier `json:"tier"`
	Path       string      `json:"path"`
	Content    string      `json:"content"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

// TopicSummary represents a theme/topic level summary that spans multiple documents
type TopicSummary struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`         // Topic name/title
	Description string   `json:"description"`  // Brief description
	Summary     string   `json:"summary"`      // LLM-generated summary
	Tags        []string `json:"tags"`         // Related tags
	DocumentIDs []string `json:"document_ids"` // Associated document IDs
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DocumentAnalysisResult holds LLM analysis results for a document
type DocumentAnalysisResult struct {
	Summary     string   `json:"summary"`     // Document-level summary
	Tags        []string `json:"tags"`        // Generated tags
	Topic       string   `json:"topic"`       // Suggested topic/theme
	KeyPoints   []string `json:"key_points"`  // Key takeaways
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
	CreatedAt time.Time `json:"created_at"`
}

// QueryRequest represents a query request
type QueryRequest struct {
	Question string      `form:"question" binding:"required"`
	Depth    SummaryTier `form:"depth" binding:"omitempty,oneof=topic document chapter paragraph source"`
	Tags     []string    `form:"tags,omitempty"`
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

// HierarchyNode represents a node in the document hierarchy
type HierarchyNode struct {
	Level    int              `json:"level"`
	Title    string           `json:"title"`
	Path     string           `json:"path"`
	Summary  string           `json:"summary,omitempty"`
	Children []*HierarchyNode `json:"children,omitempty"`
}

// HierarchyResponse represents the document hierarchy
type HierarchyResponse struct {
	DocumentID    string         `json:"document_id"`
	DocumentTitle string         `json:"document_title"`
	Hierarchy     *HierarchyNode `json:"hierarchy"`
}

// ParsedDocument represents a parsed document with hierarchical structure
type ParsedDocument struct {
	Title    string
	Content  string
	Root     *ParsedNode
}

// ParsedNode represents a node in the parsed document tree
type ParsedNode struct {
	Level    int           `json:"level"`
	Title    string        `json:"title"`
	Content  string        `json:"content"`
	Path     string        `json:"path"`
	Children []*ParsedNode `json:"children,omitempty"`
}
