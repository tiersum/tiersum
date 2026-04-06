// Package ports defines all interfaces for dependency inversion
// Each layer only depends on interfaces defined in this package
package ports

import (
	"context"

	"github.com/tiersum/tiersum/pkg/types"
)

// ============================================================================
// Repository Interfaces (Data Access Layer)
// ============================================================================

// DocumentRepository defines minimal document storage operations
type DocumentRepository interface {
	// Create saves a new document
	Create(ctx context.Context, doc *types.Document) error
	// GetByID retrieves a document by ID
	GetByID(ctx context.Context, id string) (*types.Document, error)
}

// SummaryRepository defines minimal summary storage operations
type SummaryRepository interface {
	// Create saves a new summary
	Create(ctx context.Context, summary *types.Summary) error
	// GetByDocument retrieves all summaries for a document
	GetByDocument(ctx context.Context, docID string) ([]types.Summary, error)
}

// TopicSummaryRepository defines topic summary storage operations
type TopicSummaryRepository interface {
	// Create saves a new topic summary
	Create(ctx context.Context, topic *types.TopicSummary) error
	// GetByID retrieves a topic summary by ID
	GetByID(ctx context.Context, id string) (*types.TopicSummary, error)
	// GetByName retrieves a topic summary by name
	GetByName(ctx context.Context, name string) (*types.TopicSummary, error)
	// List retrieves all topic summaries
	List(ctx context.Context) ([]types.TopicSummary, error)
	// FindByTags retrieves topic summaries matching any of the given tags
	FindByTags(ctx context.Context, tags []string) ([]types.TopicSummary, error)
	// AddDocument adds a document to a topic
	AddDocument(ctx context.Context, topicID string, docID string) error
	// RemoveDocument removes a document from a topic
	RemoveDocument(ctx context.Context, topicID string, docID string) error
}

// Cache defines minimal cache operations
type Cache interface {
	// Get retrieves value by key
	Get(key string) (interface{}, bool)
	// Set stores value
	Set(key string, value interface{})
}

// ============================================================================
// Service Interfaces (Business Logic Layer)
// ============================================================================

// DocumentService defines document business logic
type DocumentService interface {
	// Ingest processes and stores a new document
	Ingest(ctx context.Context, req types.CreateDocumentRequest) (*types.Document, error)
	// Get retrieves a document by ID
	Get(ctx context.Context, id string) (*types.Document, error)
}

// QueryService defines query business logic
type QueryService interface {
	// Query performs hierarchical query
	Query(ctx context.Context, question string, depth types.SummaryTier) ([]types.QueryResult, error)
}

// ============================================================================
// Core Interfaces (Domain Logic Layer)
// ============================================================================

// Parser defines document parsing capability
type Parser interface {
	// Parse parses content into hierarchical document
	Parse(content string) (*types.ParsedDocument, error)
}

// Summarizer defines summarization capability
type Summarizer interface {
	// Summarize creates summary for given content
	Summarize(ctx context.Context, content string, level types.SummaryTier) (string, error)
	
	// AnalyzeDocument performs full document analysis including summary and tags
	AnalyzeDocument(ctx context.Context, title string, content string) (*types.DocumentAnalysisResult, error)
	
	// GenerateTopicSummary creates a topic-level summary from multiple documents
	GenerateTopicSummary(ctx context.Context, topicName string, documents []*types.Document) (*types.TopicSummary, error)
}

// Indexer defines document indexing capability
type Indexer interface {
	// Index processes and indexes a document
	Index(ctx context.Context, docID string, content string) error
}

// ============================================================================
// Infrastructure Interfaces
// ============================================================================

// LLMProvider defines LLM service interface
type LLMProvider interface {
	// Generate generates text completion
	Generate(ctx context.Context, prompt string, maxTokens int) (string, error)
}
