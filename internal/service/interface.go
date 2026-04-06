// Package service defines business logic interfaces
// Service layer interfaces with I-prefix naming convention
package service

import (
	"context"

	"github.com/tiersum/tiersum/pkg/types"
)

// IDocumentService defines document business logic
type IDocumentService interface {
	// Ingest processes and stores a new document
	// Automatically generates tags, summary, and chapter summaries
	Ingest(ctx context.Context, req types.CreateDocumentRequest) (*types.CreateDocumentResponse, error)
	// Get retrieves a document by ID
	Get(ctx context.Context, id string) (*types.Document, error)
}

// IQueryService defines query business logic
type IQueryService interface {
	// Query performs hierarchical query with LLM filtering
	Query(ctx context.Context, question string, depth types.SummaryTier) ([]types.QueryResult, error)
	// ProgressiveQuery performs the new two-level tag-based progressive query
	ProgressiveQuery(ctx context.Context, req types.ProgressiveQueryRequest) (*types.ProgressiveQueryResponse, error)
}

// ITagGroupService defines tag grouping business logic
type ITagGroupService interface {
	// GroupTags performs LLM-based grouping of all global tags
	// Creates Level 1 groups from Level 2 tags
	GroupTags(ctx context.Context) error
	// ShouldRefresh checks if grouping should be performed based on tag count change
	ShouldRefresh(ctx context.Context) (bool, error)
	// GetL1Groups retrieves all Level 1 groups
	GetL1Groups(ctx context.Context) ([]types.TagGroup, error)
	// GetL2TagsByGroup retrieves Level 2 tags belonging to a group
	GetL2TagsByGroup(ctx context.Context, groupID string) ([]types.Tag, error)
	// FilterL2TagsByQuery uses LLM to filter L2 tags based on query
	FilterL2TagsByQuery(ctx context.Context, query string, tags []types.Tag) ([]types.TagFilterResult, error)
}

// IIndexer defines document indexing logic
type IIndexer interface {
	// Index processes and indexes a document
	// Creates document summary, chapter summaries, and stores source content
	Index(ctx context.Context, doc *types.Document, analysis *types.DocumentAnalysisResult) error
}

// ISummarizer defines summarization logic
type ISummarizer interface {
	// AnalyzeDocument performs full document analysis
	// Returns document summary, tags (max 10), and chapter summaries
	AnalyzeDocument(ctx context.Context, title string, content string) (*types.DocumentAnalysisResult, error)
	// FilterDocuments selects relevant documents based on query
	FilterDocuments(ctx context.Context, query string, docs []types.Document) ([]types.LLMFilterResult, error)
	// FilterChapters selects relevant chapters based on query
	FilterChapters(ctx context.Context, query string, chapters []types.Summary) ([]types.LLMFilterResult, error)
}

// ILLMFilter is kept for backward compatibility, merged into ISummarizer
type ILLMFilter interface {
	// FilterDocuments selects relevant documents based on query
	FilterDocuments(ctx context.Context, query string, docs []types.Document) ([]types.LLMFilterResult, error)
	// FilterChapters selects relevant chapters based on query
	FilterChapters(ctx context.Context, query string, chapters []types.Summary) ([]types.LLMFilterResult, error)
}
