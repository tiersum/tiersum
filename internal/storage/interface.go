// Package storage defines storage layer interfaces
package storage

import (
	"context"

	"github.com/tiersum/tiersum/pkg/types"
)

// IDocumentRepository defines document storage operations
type IDocumentRepository interface {
	Create(ctx context.Context, doc *types.Document) error
	GetByID(ctx context.Context, id string) (*types.Document, error)
	// GetRecent retrieves recent documents up to a limit
	GetRecent(ctx context.Context, limit int) ([]*types.Document, error)
	// ListByTags retrieves documents that match ANY of the given tags (OR logic)
	ListByTags(ctx context.Context, tags []string, limit int) ([]types.Document, error)
	// ListByStatus retrieves documents by status (hot/cold/warming)
	ListByStatus(ctx context.Context, status types.DocumentStatus, limit int) ([]types.Document, error)
	// IncrementQueryCount increments the query count and updates last_query_at
	IncrementQueryCount(ctx context.Context, docID string) error
	// UpdateStatus updates the document status (hot/cold/warming)
	UpdateStatus(ctx context.Context, docID string, status types.DocumentStatus) error
	// UpdateHotScore updates the hot score for a document
	UpdateHotScore(ctx context.Context, docID string, score float64) error
	// ListAll returns all documents for hot score calculation
	ListAll(ctx context.Context, limit int) ([]types.Document, error)
}

// ISummaryRepository defines summary storage operations
type ISummaryRepository interface {
	Create(ctx context.Context, summary *types.Summary) error
	GetByDocument(ctx context.Context, docID string) ([]types.Summary, error)
	// GetByPath retrieves a summary by its exact path
	GetByPath(ctx context.Context, path string) (*types.Summary, error)
	// QueryByTierAndPrefix queries summaries by tier and path prefix
	// Used for hierarchical queries: e.g., tier=chapter, prefix="doc_001" returns all chapters of doc_001
	QueryByTierAndPrefix(ctx context.Context, tier types.SummaryTier, pathPrefix string) ([]types.Summary, error)
	// DeleteByDocument removes all summaries for a document (useful for re-indexing)
	DeleteByDocument(ctx context.Context, docID string) error
}

// ITagRepository defines global tag storage operations
type ITagRepository interface {
	// Create creates a new global tag
	Create(ctx context.Context, tag *types.Tag) error
	// GetByName retrieves a tag by name
	GetByName(ctx context.Context, name string) (*types.Tag, error)
	// List retrieves all global tags
	List(ctx context.Context) ([]types.Tag, error)
	// ListByGroup retrieves tags belonging to a specific group
	ListByGroup(ctx context.Context, groupID string) ([]types.Tag, error)
	// IncrementDocumentCount increments the document count for a tag
	IncrementDocumentCount(ctx context.Context, tagName string) error
	// DeleteAll removes all global tags (used before re-grouping)
	DeleteAll(ctx context.Context) error
	// GetCount returns the total number of global tags
	GetCount(ctx context.Context) (int, error)
}

// ITagGroupRepository defines tag group storage operations
type ITagGroupRepository interface {
	// Create creates a new tag group
	Create(ctx context.Context, group *types.TagGroup) error
	// GetByID retrieves a group by ID
	GetByID(ctx context.Context, id string) (*types.TagGroup, error)
	// List retrieves all tag groups
	List(ctx context.Context) ([]types.TagGroup, error)
	// DeleteAll removes all groups (used before re-grouping)
	DeleteAll(ctx context.Context) error
	// GetCount returns the total number of groups
	GetCount(ctx context.Context) (int, error)
}

// ICache defines cache operations
type ICache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
}

// IInMemoryIndex defines in-memory index operations for cold documents
type IInMemoryIndex interface {
	// AddDocument adds a document to the index with optional embedding
	AddDocument(doc *types.Document, embedding []float32) error
	// RemoveDocument removes a document from the index
	RemoveDocument(docID string) error
	// Search performs hybrid BM25 + vector search
	Search(ctx context.Context, queryText string, queryEmbedding []float32, topK int) ([]SearchResult, error)
	// SearchWithBleve performs BM25 text search only
	SearchWithBleve(queryText string, topK int) ([]SearchResult, error)
	// SearchWithVector performs vector similarity search only
	SearchWithVector(queryEmbedding []float32, topK int) ([]SearchResult, error)
	// HybridSearch performs hybrid search with result merging
	HybridSearch(queryText string, queryEmbedding []float32, topK int) ([]SearchResult, error)
	// GetDocumentCount returns the number of indexed documents
	GetDocumentCount() int
	// RebuildFromDocuments rebuilds the index from a list of documents
	RebuildFromDocuments(ctx context.Context, docs []types.Document, getEmbedding func(doc *types.Document) []float32) error
	// Close closes the index
	Close() error
}

// SearchResult represents a search result from in-memory index
type SearchResult struct {
	DocumentID string  `json:"document_id"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	Source     string  `json:"source"` // "bm25", "vector", or "hybrid"
}
