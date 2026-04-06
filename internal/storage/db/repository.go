// Package db implements database storage layer
package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// sqlDB is a minimal interface for database operations
type sqlDB interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

// DocumentRepo implements storage.IDocumentRepository
type DocumentRepo struct {
	db     sqlDB
	driver string
	cache  storage.ICache
}

// NewDocumentRepo creates a new document repository
func NewDocumentRepo(db sqlDB, driver string, cache storage.ICache) *DocumentRepo {
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

	query := `INSERT INTO documents (id, title, content, format, tags, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	if r.driver == "postgres" {
		query = `INSERT INTO documents (id, title, content, format, tags, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	}

	_, err := r.db.ExecContext(ctx, query, doc.ID, doc.Title, doc.Content, doc.Format, doc.Tags, doc.CreatedAt, doc.UpdatedAt)
	return err
}

// GetByID implements IDocumentRepository.GetByID
func (r *DocumentRepo) GetByID(ctx context.Context, id string) (*types.Document, error) {
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

	if r.cache != nil {
		r.cache.Set("doc:"+id, doc)
	}
	return doc, nil
}

var _ storage.IDocumentRepository = (*DocumentRepo)(nil)

// SummaryRepo implements storage.ISummaryRepository
type SummaryRepo struct {
	db     sqlDB
	driver string
	cache  storage.ICache
}

// NewSummaryRepo creates a new summary repository
func NewSummaryRepo(db sqlDB, driver string, cache storage.ICache) *SummaryRepo {
	return &SummaryRepo{
		db:     db,
		driver: driver,
		cache:  cache,
	}
}

// Create implements ISummaryRepository.Create
func (r *SummaryRepo) Create(ctx context.Context, summary *types.Summary) error {
	if summary.ID == "" {
		summary.ID = uuid.New().String()
	}
	now := time.Now()
	summary.CreatedAt = now
	summary.UpdatedAt = now

	query := `INSERT INTO summaries (id, document_id, tier, path, content, is_source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	if r.driver == "postgres" {
		query = `INSERT INTO summaries (id, document_id, tier, path, content, is_source, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	}

	_, err := r.db.ExecContext(ctx, query,
		summary.ID, summary.DocumentID, summary.Tier, summary.Path, summary.Content, summary.IsSource, summary.CreatedAt, summary.UpdatedAt)
	return err
}

// GetByDocument implements ISummaryRepository.GetByDocument
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

// QueryByTierAndPrefix implements ISummaryRepository.QueryByTierAndPrefix
func (r *SummaryRepo) QueryByTierAndPrefix(ctx context.Context, tier types.SummaryTier, pathPrefix string) ([]types.Summary, error) {
	query := `SELECT id, document_id, tier, path, content, is_source, created_at, updated_at FROM summaries WHERE tier = ? AND path LIKE ? ORDER BY path`
	if r.driver == "postgres" {
		query = `SELECT id, document_id, tier, path, content, is_source, created_at, updated_at FROM summaries WHERE tier = $1 AND path LIKE $2 ORDER BY path`
	}

	rows, err := r.db.QueryContext(ctx, query, tier, pathPrefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []types.Summary
	for rows.Next() {
		var s types.Summary
		if err := rows.Scan(&s.ID, &s.DocumentID, &s.Tier, &s.Path, &s.Content, &s.IsSource, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return summaries, nil
}

// GetByPath implements ISummaryRepository.GetByPath
func (r *SummaryRepo) GetByPath(ctx context.Context, path string) (*types.Summary, error) {
	query := `SELECT id, document_id, tier, path, content, is_source, created_at, updated_at FROM summaries WHERE path = ?`
	if r.driver == "postgres" {
		query = `SELECT id, document_id, tier, path, content, is_source, created_at, updated_at FROM summaries WHERE path = $1`
	}

	var s types.Summary
	err := r.db.QueryRowContext(ctx, query, path).Scan(
		&s.ID, &s.DocumentID, &s.Tier, &s.Path, &s.Content, &s.IsSource, &s.CreatedAt, &s.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetChildrenPaths implements ISummaryRepository.GetChildrenPaths
func (r *SummaryRepo) GetChildrenPaths(ctx context.Context, parentPath string, tier types.SummaryTier) ([]string, error) {
	// Children paths start with parentPath + "/"
	query := `SELECT path FROM summaries WHERE tier = ? AND path LIKE ? AND path != ? ORDER BY path`
	if r.driver == "postgres" {
		query = `SELECT path FROM summaries WHERE tier = $1 AND path LIKE $2 AND path != $3 ORDER BY path`
	}

	rows, err := r.db.QueryContext(ctx, query, tier, parentPath+"/%", parentPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}

var _ storage.ISummaryRepository = (*SummaryRepo)(nil)

// TopicSummaryRepo implements storage.ITopicSummaryRepository
type TopicSummaryRepo struct {
	db     sqlDB
	driver string
	cache  storage.ICache
}

// NewTopicSummaryRepo creates a new topic summary repository
func NewTopicSummaryRepo(db sqlDB, driver string, cache storage.ICache) *TopicSummaryRepo {
	return &TopicSummaryRepo{
		db:     db,
		driver: driver,
		cache:  cache,
	}
}

// Create implements ITopicSummaryRepository.Create
func (r *TopicSummaryRepo) Create(ctx context.Context, topic *types.TopicSummary) error {
	if topic.ID == "" {
		topic.ID = uuid.New().String()
	}
	now := time.Now()
	topic.CreatedAt = now
	topic.UpdatedAt = now

	// Default to manual if source not set
	if topic.Source == "" {
		topic.Source = types.TopicSourceManual
	}

	query := `INSERT INTO topic_summaries (id, name, description, summary, tags, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	if r.driver == "postgres" {
		query = `INSERT INTO topic_summaries (id, name, description, summary, tags, source, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	}

	_, err := r.db.ExecContext(ctx, query,
		topic.ID, topic.Name, topic.Description, topic.Summary, topic.Tags, topic.Source, topic.CreatedAt, topic.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create topic summary: %w", err)
	}

	for _, docID := range topic.DocumentIDs {
		if err := r.AddDocument(ctx, topic.ID, docID); err != nil {
			return fmt.Errorf("add document to topic: %w", err)
		}
	}

	return nil
}

// GetByID implements ITopicSummaryRepository.GetByID
func (r *TopicSummaryRepo) GetByID(ctx context.Context, id string) (*types.TopicSummary, error) {
	cacheKey := "topic:" + id
	if r.cache != nil {
		if cached, ok := r.cache.Get(cacheKey); ok {
			return cached.(*types.TopicSummary), nil
		}
	}

	query := `SELECT id, name, description, summary, tags, source, created_at, updated_at FROM topic_summaries WHERE id = ?`
	if r.driver == "postgres" {
		query = `SELECT id, name, description, summary, tags, source, created_at, updated_at FROM topic_summaries WHERE id = $1`
	}

	topic := &types.TopicSummary{}
	var tagsStr, sourceStr string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&topic.ID, &topic.Name, &topic.Description, &topic.Summary, &tagsStr, &sourceStr, &topic.CreatedAt, &topic.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get topic by id: %w", err)
	}

	topic.Tags = parseStringArray(tagsStr)
	topic.Source = types.TopicSource(sourceStr)

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

// GetByName implements ITopicSummaryRepository.GetByName
func (r *TopicSummaryRepo) GetByName(ctx context.Context, name string) (*types.TopicSummary, error) {
	query := `SELECT id, name, description, summary, tags, source, created_at, updated_at FROM topic_summaries WHERE name = ?`
	if r.driver == "postgres" {
		query = `SELECT id, name, description, summary, tags, source, created_at, updated_at FROM topic_summaries WHERE name = $1`
	}

	topic := &types.TopicSummary{}
	var tagsStr, sourceStr string
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&topic.ID, &topic.Name, &topic.Description, &topic.Summary, &tagsStr, &sourceStr, &topic.CreatedAt, &topic.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get topic by name: %w", err)
	}

	topic.Tags = parseStringArray(tagsStr)
	topic.Source = types.TopicSource(sourceStr)

	docIDs, err := r.getDocumentIDs(ctx, topic.ID)
	if err != nil {
		return nil, err
	}
	topic.DocumentIDs = docIDs

	return topic, nil
}

// List implements ITopicSummaryRepository.List
func (r *TopicSummaryRepo) List(ctx context.Context) ([]types.TopicSummary, error) {
	cacheKey := "topics:all"
	if r.cache != nil {
		if cached, ok := r.cache.Get(cacheKey); ok {
			return cached.([]types.TopicSummary), nil
		}
	}

	query := `SELECT id, name, description, summary, tags, source, created_at, updated_at FROM topic_summaries ORDER BY name`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	defer rows.Close()

	var topics []types.TopicSummary
	for rows.Next() {
		var t types.TopicSummary
		var tagsStr, sourceStr string
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Summary, &tagsStr, &sourceStr, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.Tags = parseStringArray(tagsStr)
		t.Source = types.TopicSource(sourceStr)
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

// FindByTags implements ITopicSummaryRepository.FindByTags
func (r *TopicSummaryRepo) FindByTags(ctx context.Context, tags []string) ([]types.TopicSummary, error) {
	if len(tags) == 0 {
		return r.List(ctx)
	}

	var query string
	if r.driver == "postgres" {
		query = `SELECT id, name, description, summary, tags, source, created_at, updated_at FROM topic_summaries WHERE tags && $1 ORDER BY name`
	} else {
		query = `SELECT id, name, description, summary, tags, source, created_at, updated_at FROM topic_summaries WHERE `
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
		var tagsStr, sourceStr string
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Summary, &tagsStr, &sourceStr, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.Tags = parseStringArray(tagsStr)
		t.Source = types.TopicSource(sourceStr)
		topics = append(topics, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return topics, nil
}

// AddDocument implements ITopicSummaryRepository.AddDocument
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

// RemoveDocument implements ITopicSummaryRepository.RemoveDocument
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

// GetTopicDocuments implements ITopicSummaryRepository.GetTopicDocuments
func (r *TopicSummaryRepo) GetTopicDocuments(ctx context.Context, topicID string) ([]types.Document, error) {
	docIDs, err := r.getDocumentIDs(ctx, topicID)
	if err != nil {
		return nil, err
	}

	if len(docIDs) == 0 {
		return []types.Document{}, nil
	}

	// Build IN clause
	placeholders := make([]string, len(docIDs))
	args := make([]interface{}, len(docIDs))
	for i, id := range docIDs {
		if r.driver == "postgres" {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		} else {
			placeholders[i] = "?"
		}
		args[i] = id
	}

	query := fmt.Sprintf(`SELECT id, title, content, format, tags, created_at, updated_at FROM documents WHERE id IN (%s)`, 
		strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get topic documents: %w", err)
	}
	defer rows.Close()

	var documents []types.Document
	for rows.Next() {
		var d types.Document
		var tagsStr string
		if err := rows.Scan(&d.ID, &d.Title, &d.Content, &d.Format, &tagsStr, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		d.Tags = parseStringArray(tagsStr)
		documents = append(documents, d)
	}
	return documents, rows.Err()
}

var _ storage.ITopicSummaryRepository = (*TopicSummaryRepo)(nil)

func parseStringArray(s string) []string {
	if s == "" || s == "{}" {
		return []string{}
	}
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

// UnitOfWork combines multiple repositories
type UnitOfWork struct {
	Documents      storage.IDocumentRepository
	Summaries      storage.ISummaryRepository
	TopicSummaries storage.ITopicSummaryRepository
}

// NewUnitOfWork creates a new unit of work
func NewUnitOfWork(db sqlDB, driver string, cache storage.ICache) *UnitOfWork {
	return &UnitOfWork{
		Documents:      NewDocumentRepo(db, driver, cache),
		Summaries:      NewSummaryRepo(db, driver, cache),
		TopicSummaries: NewTopicSummaryRepo(db, driver, cache),
	}
}
