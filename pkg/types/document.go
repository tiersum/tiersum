package types

import (
	"time"

	"github.com/google/uuid"
)

// Document represents a document in the system
type Document struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Format    string    `json:"format"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SummaryTier represents the level of summarization
type SummaryTier string

const (
	TierDocument  SummaryTier = "document"
	TierChapter   SummaryTier = "chapter"
	TierParagraph SummaryTier = "paragraph"
	TierSource    SummaryTier = "source"
)

// Summary represents a summary at a specific tier
type Summary struct {
	ID         uuid.UUID   `json:"id"`
	DocumentID uuid.UUID   `json:"document_id"`
	Tier       SummaryTier `json:"tier"`
	Path       string      `json:"path"`
	Content    string      `json:"content"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
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
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Format    string    `json:"format"`
	CreatedAt time.Time `json:"created_at"`
}

// QueryRequest represents a query request
type QueryRequest struct {
	Question string      `form:"question" binding:"required"`
	Depth    SummaryTier `form:"depth" binding:"omitempty,oneof=document chapter paragraph source"`
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
	DocumentID    uuid.UUID   `json:"document_id"`
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
	DocumentID    uuid.UUID      `json:"document_id"`
	DocumentTitle string         `json:"document_title"`
	Hierarchy     *HierarchyNode `json:"hierarchy"`
}
