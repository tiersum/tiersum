// Package db implements database storage layer
// Database access implementations
package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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

	query := `INSERT INTO documents (id, title, content, format, tags, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	if r.driver == "postgres" {
		query = `INSERT INTO documents (id, title, content, format, tags, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	}

	_, err := r.db.ExecContext(ctx, query, doc.ID, doc.Title, doc.Content, doc.Format, doc.Tags, doc.CreatedAt, doc.UpdatedAt)
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

	query := `SELECT id, title, content, format, tags, created_at, updated_at FROM documents WHERE id = ?`
	if r.driver == "postgres" {
		query = `SELECT id, title, content, format, tags, created_at, updated_at FROM documents WHERE id = $1`
	}

	doc := &types.Document{}
	var tagsStr string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&doc.ID, &doc.Title, &doc.Content, &doc.Format, &tagsStr, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err == nil {
		doc.Tags = parseStringArray(tagsStr)
	}
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

// TopicSummaryRepo implements ports.TopicSummaryRepository
type TopicSummaryRepo struct {
	db     sqlDB
	driver string
	cache  ports.Cache
}

// NewTopicSummaryRepo creates a new topic summary repository
func NewTopicSummaryRepo(db sqlDB, driver string, cache ports.Cache) *TopicSummaryRepo {
	return &TopicSummaryRepo{
		db:     db,
		driver: driver,
		cache:  cache,
	}
}

// Create implements ports.TopicSummaryRepository.Create
func (r *TopicSummaryRepo) Create(ctx context.Context, topic *types.TopicSummary) error {
	if topic.ID == "" {
		topic.ID = uuid.New().String()
	}
	now := time.Now()
	topic.CreatedAt = now
	topic.UpdatedAt = now

	query := `INSERT INTO topic_summaries (id, name, description, summary, tags, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	if r.driver == "postgres" {
		query = `INSERT INTO topic_summaries (id, name, description, summary, tags, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	}

	_, err := r.db.ExecContext(ctx, query,
		topic.ID, topic.Name, topic.Description, topic.Summary, topic.Tags, topic.CreatedAt, topic.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create topic summary: %w", err)
	}

	// Insert document relationships
	for _, docID := range topic.DocumentIDs {
		if err := r.AddDocument(ctx, topic.ID, docID); err != nil {
			return fmt.Errorf("add document to topic: %w", err)
		}
	}

	return nil
}

// GetByID implements ports.TopicSummaryRepository.GetByID
func (r *TopicSummaryRepo) GetByID(ctx context.Context, id string) (*types.TopicSummary, error) {
	cacheKey := "topic:" + id
	if r.cache != nil {
		if cached, ok := r.cache.Get(cacheKey); ok {
			return cached.(*types.TopicSummary), nil
		}
	}

	query := `SELECT id, name, description, summary, tags, created_at, updated_at FROM topic_summaries WHERE id = ?`
	if r.driver == "postgres" {
		query = `SELECT id, name, description, summary, tags, created_at, updated_at FROM topic_summaries WHERE id = $1`
	}

	topic := &types.TopicSummary{}
	var tagsStr string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&topic.ID, &topic.Name, &topic.Description, &topic.Summary, &tagsStr, &topic.CreatedAt, &topic.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get topic by id: %w", err)
	}

	topic.Tags = parseStringArray(tagsStr)

	// Get associated documents
	docIDs, err := r.getDocumentIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	topic.DocumentIDs = docIDs

	if r.cache != nil {
		r.cache.Set(cacheKey, topic)
	}
	return topic, nil
}

// GetByName implements ports.TopicSummaryRepository.GetByName
func (r *TopicSummaryRepo) GetByName(ctx context.Context, name string) (*types.TopicSummary, error) {
	query := `SELECT id, name, description, summary, tags, created_at, updated_at FROM topic_summaries WHERE name = ?`
	if r.driver == "postgres" {
		query = `SELECT id, name, description, summary, tags, created_at, updated_at FROM topic_summaries WHERE name = $1`
	}

	topic := &types.TopicSummary{}
	var tagsStr string
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&topic.ID, &topic.Name, &topic.Description, &topic.Summary, &tagsStr, &topic.CreatedAt, &topic.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get topic by name: %w", err)
	}

	topic.Tags = parseStringArray(tagsStr)

	docIDs, err := r.getDocumentIDs(ctx, topic.ID)
	if err != nil {
		return nil, err
	}
	topic.DocumentIDs = docIDs

	return topic, nil
}

// List implements ports.TopicSummaryRepository.List
func (r *TopicSummaryRepo) List(ctx context.Context) ([]types.TopicSummary, error) {
	cacheKey := "topics:all"
	if r.cache != nil {
		if cached, ok := r.cache.Get(cacheKey); ok {
			return cached.([]types.TopicSummary), nil
		}
	}

	query := `SELECT id, name, description, summary, tags, created_at, updated_at FROM topic_summaries ORDER BY name`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	defer rows.Close()

	var topics []types.TopicSummary
	for rows.Next() {
		var t types.TopicSummary
		var tagsStr string
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Summary, &tagsStr, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.Tags = parseStringArray(tagsStr)
		topics = append(topics, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if r.cache != nil {
		r.cache.Set(cacheKey, topics)
	}
	return topics, nil
}

// FindByTags implements ports.TopicSummaryRepository.FindByTags
func (r *TopicSummaryRepo) FindByTags(ctx context.Context, tags []string) ([]types.TopicSummary, error) {
	if len(tags) == 0 {
		return r.List(ctx)
	}

	// Build query with tag matching
	var query string
	if r.driver == "postgres" {
		query = `SELECT id, name, description, summary, tags, created_at, updated_at FROM topic_summaries WHERE tags && $1 ORDER BY name`
	} else {
		// SQLite doesn't have array overlap operator, use LIKE for each tag
		query = `SELECT id, name, description, summary, tags, created_at, updated_at FROM topic_summaries WHERE `
		for i := range tags {
			if i > 0 {
				query += " OR "
			}
			query += fmt.Sprintf("tags LIKE '%%%s%%'", tags[i])
		}
		query += " ORDER BY name"
	}

	var rows *sql.Rows
	var err error
	if r.driver == "postgres" {
		rows, err = r.db.QueryContext(ctx, query, tags)
	} else {
		rows, err = r.db.QueryContext(ctx, query)
	}
	if err != nil {
		return nil, fmt.Errorf("find topics by tags: %w", err)
	}
	defer rows.Close()

	var topics []types.TopicSummary
	for rows.Next() {
		var t types.TopicSummary
		var tagsStr string
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Summary, &tagsStr, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.Tags = parseStringArray(tagsStr)
		topics = append(topics, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return topics, nil
}

// AddDocument implements ports.TopicSummaryRepository.AddDocument
func (r *TopicSummaryRepo) AddDocument(ctx context.Context, topicID string, docID string) error {
	query := `INSERT INTO topic_documents (topic_id, document_id) VALUES (?, ?) ON CONFLICT DO NOTHING`
	if r.driver == "postgres" {
		query = `INSERT INTO topic_documents (topic_id, document_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	}

	_, err := r.db.ExecContext(ctx, query, topicID, docID)
	if err != nil {
		return fmt.Errorf("add document to topic: %w", err)
	}
	return nil
}

// RemoveDocument implements ports.TopicSummaryRepository.RemoveDocument
func (r *TopicSummaryRepo) RemoveDocument(ctx context.Context, topicID string, docID string) error {
	query := `DELETE FROM topic_documents WHERE topic_id = ? AND document_id = ?`
	if r.driver == "postgres" {
		query = `DELETE FROM topic_documents WHERE topic_id = $1 AND document_id = $2`
	}

	_, err := r.db.ExecContext(ctx, query, topicID, docID)
	if err != nil {
		return fmt.Errorf("remove document from topic: %w", err)
	}
	return nil
}

// getDocumentIDs retrieves all document IDs associated with a topic
func (r *TopicSummaryRepo) getDocumentIDs(ctx context.Context, topicID string) ([]string, error) {
	query := `SELECT document_id FROM topic_documents WHERE topic_id = ?`
	if r.driver == "postgres" {
		query = `SELECT document_id FROM topic_documents WHERE topic_id = $1`
	}

	rows, err := r.db.QueryContext(ctx, query, topicID)
	if err != nil {
		return nil, fmt.Errorf("get document ids: %w", err)
	}
	defer rows.Close()

	var docIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		docIDs = append(docIDs, id)
	}
	return docIDs, rows.Err()
}

// parseStringArray parses a string array from database
func parseStringArray(s string) []string {
	if s == "" || s == "{}" {
		return []string{}
	}
	// Remove PostgreSQL array notation {a,b,c}
	s = strings.Trim(s, "{}")
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	result := make([]string, len(parts))
	for i, p := range parts {
		result[i] = strings.Trim(p, "\"")
	}
	return result
}

// UnitOfWork combines multiple repositories for transactional operations
type UnitOfWork struct {
	Documents      ports.DocumentRepository
	Summaries      ports.SummaryRepository
	TopicSummaries ports.TopicSummaryRepository
}

// NewUnitOfWork creates a new unit of work
func NewUnitOfWork(db sqlDB, driver string, cache ports.Cache) *UnitOfWork {
	return &UnitOfWork{
		Documents:      NewDocumentRepo(db, driver, cache),
		Summaries:      NewSummaryRepo(db, driver, cache),
		TopicSummaries: NewTopicSummaryRepo(db, driver, cache),
	}
}

// Compile-time interface checks
var (
	_ ports.DocumentRepository     = (*DocumentRepo)(nil)
	_ ports.SummaryRepository      = (*SummaryRepo)(nil)
	_ ports.TopicSummaryRepository = (*TopicSummaryRepo)(nil)
)
