package storage

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Document represents a stored document
type Document struct {
	ID        string    `json:"id" db:"id"`
	Title     string    `json:"title" db:"title"`
	Content   string    `json:"content" db:"content"`
	Format    string    `json:"format" db:"format"`
	Tags      []string  `json:"tags,omitempty" db:"tags"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Summary represents a summary at a specific tier
type Summary struct {
	ID         string    `json:"id" db:"id"`
	DocumentID string    `json:"document_id" db:"document_id"`
	Tier       string    `json:"tier" db:"tier"` // document, chapter, paragraph
	Path       string    `json:"path" db:"path"` // hierarchical path like "1.2.3"
	Content    string    `json:"content" db:"content"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// CreateDocument creates a new document
func (s *Storage) CreateDocument(ctx context.Context, doc *Document) error {
	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}

	query := `
		INSERT INTO documents (id, title, content, format, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	if s.DB.Driver() == "postgres" {
		query = `
			INSERT INTO documents (id, title, content, format, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`
	}

	_, err := s.DB.ExecContext(ctx, query, doc.ID, doc.Title, doc.Content, doc.Format, time.Now(), time.Now())
	return err
}

// GetDocument retrieves a document by ID
func (s *Storage) GetDocument(ctx context.Context, id string) (*Document, error) {
	// Try cache first
	if cached, ok := s.Cache.Get("doc:" + id); ok {
		return cached.(*Document), nil
	}

	query := `SELECT id, title, content, format, created_at, updated_at FROM documents WHERE id = ?`
	if s.DB.Driver() == "postgres" {
		query = `SELECT id, title, content, format, created_at, updated_at FROM documents WHERE id = $1`
	}

	doc := &Document{}
	err := s.DB.QueryRowContext(ctx, query, id).Scan(
		&doc.ID, &doc.Title, &doc.Content, &doc.Format, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Cache the result
	s.Cache.Set("doc:"+id, doc)
	return doc, nil
}

// ListDocuments lists all documents
func (s *Storage) ListDocuments(ctx context.Context, limit, offset int) ([]*Document, error) {
	if limit == 0 {
		limit = 20
	}

	query := `SELECT id, title, content, format, created_at, updated_at FROM documents ORDER BY created_at DESC LIMIT ? OFFSET ?`
	if s.DB.Driver() == "postgres" {
		query = `SELECT id, title, content, format, created_at, updated_at FROM documents ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	}

	rows, err := s.DB.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []*Document
	for rows.Next() {
		doc := &Document{}
		if err := rows.Scan(&doc.ID, &doc.Title, &doc.Content, &doc.Format, &doc.CreatedAt, &doc.UpdatedAt); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

// CreateSummary creates a new summary
func (s *Storage) CreateSummary(ctx context.Context, summary *Summary) error {
	if summary.ID == "" {
		summary.ID = uuid.New().String()
	}

	query := `
		INSERT INTO summaries (id, document_id, tier, path, content, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	if s.DB.Driver() == "postgres" {
		query = `
			INSERT INTO summaries (id, document_id, tier, path, content, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`
	}

	_, err := s.DB.ExecContext(ctx, query,
		summary.ID, summary.DocumentID, summary.Tier, summary.Path, summary.Content, time.Now(), time.Now())
	return err
}

// GetSummariesByDocument retrieves all summaries for a document
func (s *Storage) GetSummariesByDocument(ctx context.Context, docID string) ([]*Summary, error) {
	// Try cache first
	if cached, ok := s.Cache.Get("sums:" + docID); ok {
		return cached.([]*Summary), nil
	}

	query := `SELECT id, document_id, tier, path, content, created_at, updated_at FROM summaries WHERE document_id = ? ORDER BY path`
	if s.DB.Driver() == "postgres" {
		query = `SELECT id, document_id, tier, path, content, created_at, updated_at FROM summaries WHERE document_id = $1 ORDER BY path`
	}

	rows, err := s.DB.QueryContext(ctx, query, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []*Summary
	for rows.Next() {
		summary := &Summary{}
		if err := rows.Scan(&summary.ID, &summary.DocumentID, &summary.Tier, &summary.Path, &summary.Content, &summary.CreatedAt, &summary.UpdatedAt); err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Cache the result
	s.Cache.Set("sums:"+docID, summaries)
	return summaries, nil
}

// GetSummaryAtPath retrieves a summary at a specific path
func (s *Storage) GetSummaryAtPath(ctx context.Context, docID, tier, path string) (*Summary, error) {
	cacheKey := "sum:" + docID + ":" + tier + ":" + path
	if cached, ok := s.Cache.Get(cacheKey); ok {
		return cached.(*Summary), nil
	}

	query := `SELECT id, document_id, tier, path, content, created_at, updated_at FROM summaries WHERE document_id = ? AND tier = ? AND path = ?`
	if s.DB.Driver() == "postgres" {
		query = `SELECT id, document_id, tier, path, content, created_at, updated_at FROM summaries WHERE document_id = $1 AND tier = $2 AND path = $3`
	}

	summary := &Summary{}
	err := s.DB.QueryRowContext(ctx, query, docID, tier, path).Scan(
		&summary.ID, &summary.DocumentID, &summary.Tier, &summary.Path, &summary.Content, &summary.CreatedAt, &summary.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	s.Cache.Set(cacheKey, summary)
	return summary, nil
}
