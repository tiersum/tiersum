// Package repository implements repository interfaces defined in ports
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tiersum/tiersum/internal/ports"
	"github.com/tiersum/tiersum/pkg/types"
)

// sqlDB is a minimal interface for database operations
type sqlDB interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

// DocumentRepo implements ports.DocumentRepository
type DocumentRepo struct {
	db     sqlDB
	driver string
	cache  ports.Cache
}

// NewDocumentRepo creates a new document repository
func NewDocumentRepo(db sqlDB, driver string, cache ports.Cache) *DocumentRepo {
	return &DocumentRepo{
		db:     db,
		driver: driver,
		cache:  cache,
	}
}

// Create implements ports.DocumentRepository.Create
func (r *DocumentRepo) Create(ctx context.Context, doc *types.Document) error {
	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}
	now := time.Now()
	doc.CreatedAt = now
	doc.UpdatedAt = now

	query := `INSERT INTO documents (id, title, content, format, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`
	if r.driver == "postgres" {
		query = `INSERT INTO documents (id, title, content, format, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`
	}

	_, err := r.db.ExecContext(ctx, query, doc.ID, doc.Title, doc.Content, doc.Format, doc.CreatedAt, doc.UpdatedAt)
	return err
}

// GetByID implements ports.DocumentRepository.GetByID
func (r *DocumentRepo) GetByID(ctx context.Context, id string) (*types.Document, error) {
	// Try cache first
	if r.cache != nil {
		if cached, ok := r.cache.Get("doc:" + id); ok {
			return cached.(*types.Document), nil
		}
	}

	query := `SELECT id, title, content, format, created_at, updated_at FROM documents WHERE id = ?`
	if r.driver == "postgres" {
		query = `SELECT id, title, content, format, created_at, updated_at FROM documents WHERE id = $1`
	}

	doc := &types.Document{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&doc.ID, &doc.Title, &doc.Content, &doc.Format, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.cache != nil {
		r.cache.Set("doc:"+id, doc)
	}
	return doc, nil
}

// SummaryRepo implements ports.SummaryRepository
type SummaryRepo struct {
	db     sqlDB
	driver string
	cache  ports.Cache
}

// NewSummaryRepo creates a new summary repository
func NewSummaryRepo(db sqlDB, driver string, cache ports.Cache) *SummaryRepo {
	return &SummaryRepo{
		db:     db,
		driver: driver,
		cache:  cache,
	}
}

// Create implements ports.SummaryRepository.Create
func (r *SummaryRepo) Create(ctx context.Context, summary *types.Summary) error {
	if summary.ID == "" {
		summary.ID = uuid.New().String()
	}
	now := time.Now()
	summary.CreatedAt = now
	summary.UpdatedAt = now

	query := `INSERT INTO summaries (id, document_id, tier, path, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	if r.driver == "postgres" {
		query = `INSERT INTO summaries (id, document_id, tier, path, content, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	}

	_, err := r.db.ExecContext(ctx, query,
		summary.ID, summary.DocumentID, summary.Tier, summary.Path, summary.Content, summary.CreatedAt, summary.UpdatedAt)
	return err
}

// GetByDocument implements ports.SummaryRepository.GetByDocument
func (r *SummaryRepo) GetByDocument(ctx context.Context, docID string) ([]types.Summary, error) {
	cacheKey := "sums:" + docID
	if r.cache != nil {
		if cached, ok := r.cache.Get(cacheKey); ok {
			return cached.([]types.Summary), nil
		}
	}

	query := `SELECT id, document_id, tier, path, content, created_at, updated_at FROM summaries WHERE document_id = ? ORDER BY path`
	if r.driver == "postgres" {
		query = `SELECT id, document_id, tier, path, content, created_at, updated_at FROM summaries WHERE document_id = $1 ORDER BY path`
	}

	rows, err := r.db.QueryContext(ctx, query, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []types.Summary
	for rows.Next() {
		var s types.Summary
		if err := rows.Scan(&s.ID, &s.DocumentID, &s.Tier, &s.Path, &s.Content, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if r.cache != nil {
		r.cache.Set(cacheKey, summaries)
	}
	return summaries, nil
}

// UnitOfWork combines multiple repositories for transactional operations
type UnitOfWork struct {
	Documents ports.DocumentRepository
	Summaries ports.SummaryRepository
}

// NewUnitOfWork creates a new unit of work
func NewUnitOfWork(db sqlDB, driver string, cache ports.Cache) *UnitOfWork {
	return &UnitOfWork{
		Documents: NewDocumentRepo(db, driver, cache),
		Summaries: NewSummaryRepo(db, driver, cache),
	}
}

// Compile-time interface checks
var (
	_ ports.DocumentRepository = (*DocumentRepo)(nil)
	_ ports.SummaryRepository  = (*SummaryRepo)(nil)
)