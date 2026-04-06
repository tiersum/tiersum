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
	// ListByTags retrieves documents that match ANY of the given tags (OR logic)
	ListByTags(ctx context.Context, tags []string, limit int) ([]types.Document, error)
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

// IGlobalTagRepository defines global tag storage operations
type IGlobalTagRepository interface {
	// Create creates a new global tag
	Create(ctx context.Context, tag *types.GlobalTag) error
	// GetByName retrieves a tag by name
	GetByName(ctx context.Context, name string) (*types.GlobalTag, error)
	// List retrieves all global tags
	List(ctx context.Context) ([]types.GlobalTag, error)
	// ListByCluster retrieves tags belonging to a specific cluster
	ListByCluster(ctx context.Context, clusterID string) ([]types.GlobalTag, error)
	// IncrementDocumentCount increments the document count for a tag
	IncrementDocumentCount(ctx context.Context, tagName string) error
	// DeleteAll removes all global tags (used before re-clustering)
	DeleteAll(ctx context.Context) error
	// GetCount returns the total number of global tags
	GetCount(ctx context.Context) (int, error)
}

// ITagClusterRepository defines tag cluster storage operations
type ITagClusterRepository interface {
	// Create creates a new tag cluster
	Create(ctx context.Context, cluster *types.TagCluster) error
	// GetByID retrieves a cluster by ID
	GetByID(ctx context.Context, id string) (*types.TagCluster, error)
	// List retrieves all tag clusters
	List(ctx context.Context) ([]types.TagCluster, error)
	// DeleteAll removes all clusters (used before re-clustering)
	DeleteAll(ctx context.Context) error
	// GetCount returns the total number of clusters
	GetCount(ctx context.Context) (int, error)
}

// IClusterRefreshLogRepository defines cluster refresh log operations
type IClusterRefreshLogRepository interface {
	// Create creates a new log entry
	Create(ctx context.Context, tagCountBefore, tagCountAfter, clusterCount int, durationMs int64) error
	// GetLastRefresh retrieves the most recent refresh log
	GetLastRefresh(ctx context.Context) (*ClusterRefreshLog, error)
}

// ClusterRefreshLog represents a cluster refresh log entry
type ClusterRefreshLog struct {
	ID             int64
	TagCountBefore int
	TagCountAfter  int
	ClusterCount   int
	DurationMs     int
	CreatedAt      interface{}
}

// ICache defines cache operations
type ICache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
}
