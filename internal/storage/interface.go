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
}

// ISummaryRepository defines summary storage operations
type ISummaryRepository interface {
	Create(ctx context.Context, summary *types.Summary) error
	GetByDocument(ctx context.Context, docID string) ([]types.Summary, error)
	// QueryByTierAndPrefix queries summaries by tier and path prefix
	// Used for hierarchical queries: e.g., tier=chapter, prefix="doc_001" returns all chapters of doc_001
	QueryByTierAndPrefix(ctx context.Context, tier types.SummaryTier, pathPrefix string) ([]types.Summary, error)
	// GetByPath retrieves a summary by its exact path
	GetByPath(ctx context.Context, path string) (*types.Summary, error)
	// GetChildrenPaths returns all child paths for a given parent path
	// e.g., parent="doc_001/第一章" returns ["doc_001/第一章/1", "doc_001/第一章/2", ...]
	GetChildrenPaths(ctx context.Context, parentPath string, tier types.SummaryTier) ([]string, error)
}

// ITopicSummaryRepository defines topic summary storage operations
type ITopicSummaryRepository interface {
	Create(ctx context.Context, topic *types.TopicSummary) error
	GetByID(ctx context.Context, id string) (*types.TopicSummary, error)
	GetByName(ctx context.Context, name string) (*types.TopicSummary, error)
	List(ctx context.Context) ([]types.TopicSummary, error)
	FindByTags(ctx context.Context, tags []string) ([]types.TopicSummary, error)
	AddDocument(ctx context.Context, topicID string, docID string) error
	RemoveDocument(ctx context.Context, topicID string, docID string) error
	// GetTopicDocuments returns all documents associated with a topic
	GetTopicDocuments(ctx context.Context, topicID string) ([]types.Document, error)
}

// ICache defines cache operations
type ICache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
}
