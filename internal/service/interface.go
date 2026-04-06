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
	Ingest(ctx context.Context, req types.CreateDocumentRequest) (*types.Document, error)
	// Get retrieves a document by ID
	Get(ctx context.Context, id string) (*types.Document, error)
}

// IQueryService defines query business logic
type IQueryService interface {
	// Query performs hierarchical query
	Query(ctx context.Context, question string, depth types.SummaryTier) ([]types.QueryResult, error)
}

// ITopicService defines topic management business logic
type ITopicService interface {
	// CreateTopicFromDocuments creates a new topic summary from documents
	CreateTopicFromDocuments(ctx context.Context, topicName string, docIDs []string, source types.TopicSource) (*types.TopicSummary, error)
	// GetTopic retrieves a topic by ID
	GetTopic(ctx context.Context, id string) (*types.TopicSummary, error)
	// ListTopics lists all topics
	ListTopics(ctx context.Context) ([]types.TopicSummary, error)
	// FindTopicsByTags finds topics by tags
	FindTopicsByTags(ctx context.Context, tags []string) ([]types.TopicSummary, error)
	// AddDocumentToTopics adds a document to matching topics based on tag overlap
	// Returns the number of topics the document was added to
	AddDocumentToTopics(ctx context.Context, docID string, docTags []string) (int, error)
	// AutoCreateTopicFromTag creates a new topic from documents sharing a specific tag
	AutoCreateTopicFromTag(ctx context.Context, tag string, minDocs int) (*types.TopicSummary, error)
}

// IIndexer defines document indexing logic
type IIndexer interface {
	// Index processes and indexes a document
	Index(ctx context.Context, docID string, content string) error
}

// ISummarizer defines summarization logic
type ISummarizer interface {
	// Summarize creates summary for given content
	Summarize(ctx context.Context, content string, level types.SummaryTier) (string, error)
	// AnalyzeDocument performs full document analysis including summary and tags
	AnalyzeDocument(ctx context.Context, title string, content string) (*types.DocumentAnalysisResult, error)
	// GenerateTopicSummary creates a topic-level summary from multiple documents
	GenerateTopicSummary(ctx context.Context, topicName string, documents []*types.Document, source types.TopicSource) (*types.TopicSummary, error)
}

// ILLMFilter defines LLM-based filtering capabilities for hierarchical queries
type ILLMFilter interface {
	// FilterTopics selects relevant topics based on the query
	// Returns list of topic IDs sorted by relevance
	FilterTopics(ctx context.Context, query string, topics []types.TopicSummary) ([]types.LLMFilterResult, error)
	// FilterDocuments selects relevant documents based on the query
	// Returns list of document IDs sorted by relevance
	FilterDocuments(ctx context.Context, query string, docs []types.Document) ([]types.LLMFilterResult, error)
	// FilterSummaries selects relevant summaries (chapters/paragraphs) based on the query
	// Returns list of paths sorted by relevance
	FilterSummaries(ctx context.Context, query string, summaries []types.Summary, tier types.SummaryTier) ([]types.LLMFilterResult, error)
}


