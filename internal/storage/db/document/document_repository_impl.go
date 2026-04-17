// Package document implements document, chapter, tag, and topic persistence.
package document

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/db/shared"
	"github.com/tiersum/tiersum/pkg/types"
)

// DocumentRepo implements storage.IDocumentRepository
type DocumentRepo struct {
	db     shared.SQLDB
	driver string
	cache  storage.ICache
}

// NewDocumentRepo creates a new document repository
func NewDocumentRepo(db shared.SQLDB, driver string, cache storage.ICache) *DocumentRepo {
	return &DocumentRepo{
		db:     db,
		driver: driver,
		cache:  cache,
	}
}

// Create implements IDocumentRepository.Create
func (r *DocumentRepo) Create(ctx context.Context, doc *types.Document) error {
	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}
	now := time.Now()
	doc.CreatedAt = now
	doc.UpdatedAt = now

	// Set default status if not set
	if doc.Status == "" {
		doc.Status = types.DocStatusCold
	}

	query := `INSERT INTO documents (id, title, summary, content, format, tags, status, hot_score, query_count, last_query_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if r.driver == "postgres" {
		query = `INSERT INTO documents (id, title, summary, content, format, tags, status, hot_score, query_count, last_query_at, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
	}

	_, err := r.db.ExecContext(ctx, query, doc.ID, doc.Title, doc.Summary, doc.Content, doc.Format, shared.FormatStringArray(doc.Tags), doc.Status, doc.HotScore, doc.QueryCount, doc.LastQueryAt, doc.CreatedAt, doc.UpdatedAt)
	return err
}

// GetByID implements IDocumentRepository.GetByID
func (r *DocumentRepo) GetByID(ctx context.Context, id string) (*types.Document, error) {
	if r.cache != nil {
		if cached, ok := r.cache.Get("doc:" + id); ok {
			if cached == nil {
				// Cache invalidation marker: treat as miss.
			} else if doc, ok := cached.(*types.Document); ok && doc != nil {
				return doc, nil
			}
		}
	}

	query := `SELECT id, title, summary, content, format, tags, status, hot_score, query_count, last_query_at, created_at, updated_at FROM documents WHERE id = ?`
	if r.driver == "postgres" {
		query = `SELECT id, title, summary, content, format, tags, status, hot_score, query_count, last_query_at, created_at, updated_at FROM documents WHERE id = $1`
	}

	doc := &types.Document{}
	var tagsStr string
	var lastQueryAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&doc.ID, &doc.Title, &doc.Summary, &doc.Content, &doc.Format, &tagsStr, &doc.Status, &doc.HotScore, &doc.QueryCount, &lastQueryAt, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err == nil {
		doc.Tags = shared.ParseStringArray(tagsStr)
		if lastQueryAt.Valid {
			doc.LastQueryAt = &lastQueryAt.Time
		}
	}
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if r.cache != nil {
		r.cache.Set("doc:"+id, doc)
	}
	return doc, nil
}

// GetRecent implements IDocumentRepository.GetRecent
func (r *DocumentRepo) GetRecent(ctx context.Context, limit int) ([]*types.Document, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `SELECT id, title, content, format, tags, status, hot_score, query_count, last_query_at, created_at, updated_at 
			  FROM documents 
			  ORDER BY created_at DESC 
			  LIMIT ?`
	if r.driver == "postgres" {
		query = `SELECT id, title, content, format, tags, status, hot_score, query_count, last_query_at, created_at, updated_at 
				  FROM documents 
				  ORDER BY created_at DESC 
				  LIMIT $1`
	}

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent documents: %w", err)
	}
	defer rows.Close()

	var documents []*types.Document
	for rows.Next() {
		d := &types.Document{}
		var tagsStr string
		var lastQueryAt sql.NullTime
		if err := rows.Scan(&d.ID, &d.Title, &d.Content, &d.Format, &tagsStr, &d.Status, &d.HotScore, &d.QueryCount, &lastQueryAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		d.Tags = shared.ParseStringArray(tagsStr)
		if lastQueryAt.Valid {
			d.LastQueryAt = &lastQueryAt.Time
		}
		documents = append(documents, d)
	}
	return documents, rows.Err()
}

// ListByTags retrieves documents that match ANY of the given tags
func (r *DocumentRepo) ListByTags(ctx context.Context, tags []string, limit int) ([]types.Document, error) {
	if len(tags) == 0 {
		return []types.Document{}, nil
	}
	if limit <= 0 {
		limit = 100
	}

	// Build query with OR conditions for tags
	var query string
	var args []interface{}

	if r.driver == "postgres" {
		// Use PostgreSQL array overlap operator
		query = `SELECT id, title, content, format, tags, status, hot_score, query_count, last_query_at, created_at, updated_at 
				 FROM documents 
				 WHERE tags && $1 
				 LIMIT $2`
		args = append(args, tags, limit)
	} else {
		// SQLite: Use LIKE for each tag
		conditions := make([]string, len(tags))
		for i, tag := range tags {
			conditions[i] = "tags LIKE ?"
			args = append(args, "%"+tag+"%")
		}
		query = fmt.Sprintf(`SELECT id, title, content, format, tags, status, hot_score, query_count, last_query_at, created_at, updated_at 
							 FROM documents 
							 WHERE %s 
							 LIMIT %d`,
			strings.Join(conditions, " OR "), limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query documents by tags: %w", err)
	}
	defer rows.Close()

	var documents []types.Document
	for rows.Next() {
		var d types.Document
		var tagsStr string
		var lastQueryAt sql.NullTime
		if err := rows.Scan(&d.ID, &d.Title, &d.Content, &d.Format, &tagsStr, &d.Status, &d.HotScore, &d.QueryCount, &lastQueryAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		d.Tags = shared.ParseStringArray(tagsStr)
		if lastQueryAt.Valid {
			d.LastQueryAt = &lastQueryAt.Time
		}
		documents = append(documents, d)
	}
	return documents, rows.Err()
}

// ListMetaByTagsAndStatuses returns matching documents without loading content.
func (r *DocumentRepo) ListMetaByTagsAndStatuses(ctx context.Context, tags []string, statuses []types.DocumentStatus, limit int) ([]types.Document, error) {
	if len(tags) == 0 {
		return []types.Document{}, nil
	}
	if limit <= 0 {
		limit = 1000
	}
	if len(statuses) == 0 {
		statuses = []types.DocumentStatus{types.DocStatusHot, types.DocStatusWarming}
	}

	statusList := make([]string, len(statuses))
	for i, s := range statuses {
		statusList[i] = string(s)
	}
	statusIn := "'" + strings.Join(statusList, "','") + "'"

	var query string
	var args []interface{}

	if r.driver == "postgres" {
		query = `SELECT id, title, summary, '', format, tags, status, hot_score, query_count, last_query_at, created_at, updated_at 
			FROM documents 
			WHERE tags && $1 AND status = ANY($2::text[]) 
			LIMIT $3`
		args = []interface{}{pq.Array(tags), pq.Array(statusList), limit}
	} else {
		conditions := make([]string, len(tags))
		for i, tag := range tags {
			conditions[i] = "tags LIKE ?"
			args = append(args, "%"+tag+"%")
		}
		tagWhere := strings.Join(conditions, " OR ")
		query = fmt.Sprintf(`SELECT id, title, summary, '', format, tags, status, hot_score, query_count, last_query_at, created_at, updated_at 
			FROM documents 
			WHERE (%s) AND status IN (%s) 
			LIMIT %d`,
			tagWhere, statusIn, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list document meta by tags and status: %w", err)
	}
	defer rows.Close()

	var documents []types.Document
	for rows.Next() {
		var d types.Document
		var tagsStr string
		var lastQueryAt sql.NullTime
		if err := rows.Scan(&d.ID, &d.Title, &d.Summary, &d.Content, &d.Format, &tagsStr, &d.Status, &d.HotScore, &d.QueryCount, &lastQueryAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		d.Tags = shared.ParseStringArray(tagsStr)
		if lastQueryAt.Valid {
			d.LastQueryAt = &lastQueryAt.Time
		}
		documents = append(documents, d)
	}
	return documents, rows.Err()
}

// UpdateStatus updates the document's hot/cold status
func (r *DocumentRepo) UpdateStatus(ctx context.Context, docID string, status types.DocumentStatus) error {
	query := `UPDATE documents SET status = ?, updated_at = ? WHERE id = ?`
	if r.driver == "postgres" {
		query = `UPDATE documents SET status = $1, updated_at = $2 WHERE id = $3`
	}

	_, err := r.db.ExecContext(ctx, query, status, time.Now(), docID)
	if err != nil {
		return fmt.Errorf("update document status: %w", err)
	}

	// Invalidate cache
	if r.cache != nil {
		r.cache.Set("doc:"+docID, nil)
	}
	return nil
}

// IncrementQueryCount increments the query count for a document
func (r *DocumentRepo) IncrementQueryCount(ctx context.Context, docID string) error {
	now := time.Now()
	query := `UPDATE documents SET query_count = query_count + 1, last_query_at = ?, updated_at = ? WHERE id = ?`
	if r.driver == "postgres" {
		query = `UPDATE documents SET query_count = query_count + 1, last_query_at = $1, updated_at = $2 WHERE id = $3`
	}

	_, err := r.db.ExecContext(ctx, query, now, now, docID)
	if err != nil {
		return fmt.Errorf("increment query count: %w", err)
	}

	// Invalidate cache
	if r.cache != nil {
		r.cache.Set("doc:"+docID, nil)
	}
	return nil
}

// UpdateHotScore updates the hot score for a document
func (r *DocumentRepo) UpdateHotScore(ctx context.Context, docID string, score float64) error {
	query := `UPDATE documents SET hot_score = ?, updated_at = ? WHERE id = ?`
	if r.driver == "postgres" {
		query = `UPDATE documents SET hot_score = $1, updated_at = $2 WHERE id = $3`
	}

	_, err := r.db.ExecContext(ctx, query, score, time.Now(), docID)
	if err != nil {
		return fmt.Errorf("update hot score: %w", err)
	}
	return nil
}

// UpdateTags updates document tags and updated_at.
func (r *DocumentRepo) UpdateTags(ctx context.Context, docID string, tags []string) error {
	now := time.Now()
	query := `UPDATE documents SET tags = ?, updated_at = ? WHERE id = ?`
	if r.driver == "postgres" {
		query = `UPDATE documents SET tags = $1, updated_at = $2 WHERE id = $3`
	}

	_, err := r.db.ExecContext(ctx, query, shared.FormatStringArray(tags), now, docID)
	if err != nil {
		return fmt.Errorf("update document tags: %w", err)
	}

	if r.cache != nil {
		r.cache.Set("doc:"+docID, nil)
	}
	return nil
}

// UpdateSummary updates the persisted document-level summary.
func (r *DocumentRepo) UpdateSummary(ctx context.Context, docID string, summary string) error {
	now := time.Now()
	query := `UPDATE documents SET summary = ?, updated_at = ? WHERE id = ?`
	if r.driver == "postgres" {
		query = `UPDATE documents SET summary = $1, updated_at = $2 WHERE id = $3`
	}
	_, err := r.db.ExecContext(ctx, query, summary, now, docID)
	if err != nil {
		return fmt.Errorf("update document summary: %w", err)
	}
	if r.cache != nil {
		r.cache.Set("doc:"+docID, nil)
	}
	return nil
}

// ListByStatus retrieves documents by status with optional limit
func (r *DocumentRepo) ListByStatus(ctx context.Context, status types.DocumentStatus, limit int) ([]types.Document, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `SELECT id, title, content, format, tags, status, hot_score, query_count, last_query_at, created_at, updated_at 
			  FROM documents 
			  WHERE status = ? 
			  LIMIT ?`
	if r.driver == "postgres" {
		query = `SELECT id, title, content, format, tags, status, hot_score, query_count, last_query_at, created_at, updated_at 
				  FROM documents 
				  WHERE status = $1 
				  LIMIT $2`
	}

	rows, err := r.db.QueryContext(ctx, query, status, limit)
	if err != nil {
		return nil, fmt.Errorf("query documents by status: %w", err)
	}
	defer rows.Close()

	var documents []types.Document
	for rows.Next() {
		var d types.Document
		var tagsStr string
		var lastQueryAt sql.NullTime
		if err := rows.Scan(&d.ID, &d.Title, &d.Content, &d.Format, &tagsStr, &d.Status, &d.HotScore, &d.QueryCount, &lastQueryAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		d.Tags = shared.ParseStringArray(tagsStr)
		if lastQueryAt.Valid {
			d.LastQueryAt = &lastQueryAt.Time
		}
		documents = append(documents, d)
	}
	return documents, rows.Err()
}

// ListAll returns all documents for hot score calculation
func (r *DocumentRepo) ListAll(ctx context.Context, limit int) ([]types.Document, error) {
	if limit <= 0 {
		limit = 1000
	}

	query := `SELECT id, title, content, format, tags, status, hot_score, query_count, last_query_at, created_at, updated_at 
			  FROM documents 
			  LIMIT ?`
	if r.driver == "postgres" {
		query = `SELECT id, title, content, format, tags, status, hot_score, query_count, last_query_at, created_at, updated_at 
				  FROM documents 
				  LIMIT $1`
	}

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query all documents: %w", err)
	}
	defer rows.Close()

	var documents []types.Document
	for rows.Next() {
		var d types.Document
		var tagsStr string
		var lastQueryAt sql.NullTime
		if err := rows.Scan(&d.ID, &d.Title, &d.Content, &d.Format, &tagsStr, &d.Status, &d.HotScore, &d.QueryCount, &lastQueryAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		d.Tags = shared.ParseStringArray(tagsStr)
		if lastQueryAt.Valid {
			d.LastQueryAt = &lastQueryAt.Time
		}
		documents = append(documents, d)
	}
	return documents, rows.Err()
}

// CountDocumentsByStatus implements storage.IDocumentRepository.CountDocumentsByStatus.
func (r *DocumentRepo) CountDocumentsByStatus(ctx context.Context) (types.DocumentStatusCounts, error) {
	const q = `SELECT status, COUNT(*) FROM documents GROUP BY status`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return types.DocumentStatusCounts{}, fmt.Errorf("count documents by status: %w", err)
	}
	defer rows.Close()

	var out types.DocumentStatusCounts
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return types.DocumentStatusCounts{}, err
		}
		out.Total += n
		switch types.DocumentStatus(st) {
		case types.DocStatusHot:
			out.Hot += n
		case types.DocStatusCold:
			out.Cold += n
		case types.DocStatusWarming:
			out.Warming += n
		}
	}
	return out, rows.Err()
}

var _ storage.IDocumentRepository = (*DocumentRepo)(nil)
