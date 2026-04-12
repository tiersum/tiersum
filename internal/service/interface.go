// Package service defines business logic interfaces
// Service layer interfaces with I-prefix naming convention
package service

import (
	"context"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// IDocumentService defines document business logic
type IDocumentService interface {
	// Ingest processes and stores a new document
	// Automatically generates tags, summary, and chapter summaries
	Ingest(ctx context.Context, req types.CreateDocumentRequest) (*types.CreateDocumentResponse, error)
	// Get retrieves a document by ID
	Get(ctx context.Context, id string) (*types.Document, error)
	// GetRecent retrieves recent documents up to a limit
	GetRecent(ctx context.Context, limit int) ([]*types.Document, error)
	// List retrieves all documents
	List(ctx context.Context) ([]types.Document, error)
}

// IQueryService defines query business logic
type IQueryService interface {
	// ProgressiveQuery performs the two-level tag-based progressive query
	ProgressiveQuery(ctx context.Context, req types.ProgressiveQueryRequest) (*types.ProgressiveQueryResponse, error)
}

// IDocumentMaintenanceService covers background document tiering (cold→hot promotion, hot scores).
// Used by the job layer; implementations compose storage indexing and summarization.
type IDocumentMaintenanceService interface {
	RunColdPromotionSweep(ctx context.Context) error
	PromoteColdDocumentByID(ctx context.Context, docID string) error
	RecalculateDocumentHotScores(ctx context.Context) error
}

// IHotIngestProcessor completes deferred LLM analysis and indexing for hot ingests.
// Invoked by the hot-ingest queue consumer (internal/job).
type IHotIngestProcessor interface {
	ProcessHotIngestWork(ctx context.Context, work types.HotIngestWork) error
}

// IRetrievalService exposes read operations used only by the HTTP/MCP API layer.
// It composes storage so handlers do not depend on repository or cold-index interfaces.
type IRetrievalService interface {
	ListTags(ctx context.Context, groupIDs []string, byGroupLimit int, listAllCap int) ([]types.Tag, error)
	ListSummariesForDocument(ctx context.Context, documentID string) ([]types.Summary, error)
	ListChapterSummariesForDocument(ctx context.Context, documentID string) ([]types.Summary, error)
	// MarkdownChaptersForDocument returns markdown-split sections for detail UI when DB chapter summaries are absent.
	MarkdownChaptersForDocument(ctx context.Context, doc *types.Document) ([]types.DocumentMarkdownChapter, error)
	HotDocumentsWithDocSummaries(ctx context.Context, tags []string, limit int) ([]types.Document, []types.Summary, error)
	ChapterSummariesByDocumentIDs(ctx context.Context, docIDs []string) (map[string][]types.Summary, error)
	ListSourcesByChapterPaths(ctx context.Context, paths []string) ([]types.Summary, error)
	SearchColdByQuery(ctx context.Context, query string, limit int) ([]types.ColdSearchHit, error)
	// ApproxColdIndexEntries returns the cold index size hint (chapter rows), or 0 if unavailable.
	ApproxColdIndexEntries() int
	// ColdIndexVectorStats returns HNSW / embedding monitoring fields for the cold index (zero value if unavailable).
	ColdIndexVectorStats() storage.ColdIndexVectorStats
	// ColdIndexInvertedStats returns Bleve / inverted-text monitoring fields for the cold index (zero value if unavailable).
	ColdIndexInvertedStats() storage.ColdIndexInvertedStats
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
