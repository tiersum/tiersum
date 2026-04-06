package storage

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Document represents a stored document
type Document struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Title     string    `json:"title" db:"title"`
	Content   string    `json:"content" db:"content"`
	Format    string    `json:"format" db:"format"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Summary represents a summary at a specific tier
type Summary struct {
	ID         uuid.UUID `json:"id" db:"id"`
	DocumentID uuid.UUID `json:"document_id" db:"document_id"`
	Tier       string    `json:"tier" db:"tier"` // document, chapter, paragraph
	Path       string    `json:"path" db:"path"` // hierarchical path like "1.2.3"
	Content    string    `json:"content" db:"content"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// CreateDocument creates a new document
func (s *Storage) CreateDocument(ctx context.Context, doc *Document) error {
	// TODO: Implement document creation
	return nil
}

// GetDocument retrieves a document by ID
func (s *Storage) GetDocument(ctx context.Context, id uuid.UUID) (*Document, error) {
	// TODO: Implement document retrieval
	return nil, nil
}

// ListDocuments lists all documents
func (s *Storage) ListDocuments(ctx context.Context, limit, offset int) ([]*Document, error) {
	// TODO: Implement document listing
	return nil, nil
}

// CreateSummary creates a new summary
func (s *Storage) CreateSummary(ctx context.Context, summary *Summary) error {
	// TODO: Implement summary creation
	return nil
}

// GetSummariesByDocument retrieves all summaries for a document
func (s *Storage) GetSummariesByDocument(ctx context.Context, docID uuid.UUID) ([]*Summary, error) {
	// TODO: Implement summary retrieval
	return nil, nil
}

// GetSummaryAtPath retrieves a summary at a specific path
func (s *Storage) GetSummaryAtPath(ctx context.Context, docID uuid.UUID, tier, path string) (*Summary, error) {
	// TODO: Implement path-based summary retrieval
	return nil, nil
}
