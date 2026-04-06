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
		query = `SELECT id, title, content, format, tags, created_at, updated_at 
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
		query = fmt.Sprintf(`SELECT id, title, content, format, tags, created_at, updated_at 
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
		if err := rows.Scan(&d.ID, &d.Title, &d.Content, &d.Format, &tagsStr, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		d.Tags = parseStringArray(tagsStr)
		documents = append(documents, d)
	}
	return documents, rows.Err()
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

	query := `SELECT id, document_id, tier, path, content, is_source, created_at, updated_at FROM summaries WHERE document_id = ? ORDER BY path`
	if r.driver == "postgres" {
		query = `SELECT id, document_id, tier, path, content, is_source, created_at, updated_at FROM summaries WHERE document_id = $1 ORDER BY path`
	}

	rows, err := r.db.QueryContext(ctx, query, docID)
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

	if r.cache != nil {
		r.cache.Set(cacheKey, summaries)
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

// DeleteByDocument implements ISummaryRepository.DeleteByDocument
func (r *SummaryRepo) DeleteByDocument(ctx context.Context, docID string) error {
	query := `DELETE FROM summaries WHERE document_id = ?`
	if r.driver == "postgres" {
		query = `DELETE FROM summaries WHERE document_id = $1`
	}

	_, err := r.db.ExecContext(ctx, query, docID)
	if err != nil {
		return fmt.Errorf("delete summaries by document: %w", err)
	}
	return nil
}

var _ storage.ISummaryRepository = (*SummaryRepo)(nil)

// TagRepo implements storage.ITagRepository
type TagRepo struct {
	db     sqlDB
	driver string
	cache  storage.ICache
}

// NewTagRepo creates a new global tag repository
func NewTagRepo(db sqlDB, driver string, cache storage.ICache) *TagRepo {
	return &TagRepo{
		db:     db,
		driver: driver,
		cache:  cache,
	}
}

// Create implements ITagRepository.Create
func (r *TagRepo) Create(ctx context.Context, tag *types.Tag) error {
	if tag.ID == "" {
		tag.ID = uuid.New().String()
	}
	now := time.Now()
	tag.CreatedAt = now
	tag.UpdatedAt = now

	query := `INSERT INTO global_tags (id, name, cluster_id, document_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?) 
			  ON CONFLICT(name) DO UPDATE SET updated_at = ?`
	if r.driver == "postgres" {
		query = `INSERT INTO global_tags (id, name, cluster_id, document_count, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6) 
				 ON CONFLICT(name) DO UPDATE SET updated_at = $7`
	}

	_, err := r.db.ExecContext(ctx, query, tag.ID, tag.Name, tag.GroupID, tag.DocumentCount, tag.CreatedAt, tag.UpdatedAt, tag.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create global tag: %w", err)
	}
	return nil
}

// GetByName implements ITagRepository.GetByName
func (r *TagRepo) GetByName(ctx context.Context, name string) (*types.Tag, error) {
	query := `SELECT id, name, cluster_id, document_count, created_at, updated_at FROM global_tags WHERE name = ?`
	if r.driver == "postgres" {
		query = `SELECT id, name, cluster_id, document_count, created_at, updated_at FROM global_tags WHERE name = $1`
	}

	var t types.Tag
	var clusterID sql.NullString
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&t.ID, &t.Name, &clusterID, &t.DocumentCount, &t.CreatedAt, &t.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get global tag by name: %w", err)
	}

	if clusterID.Valid {
		t.GroupID = clusterID.String
	}
	return &t, nil
}

// List implements ITagRepository.List
func (r *TagRepo) List(ctx context.Context) ([]types.Tag, error) {
	query := `SELECT id, name, cluster_id, document_count, created_at, updated_at FROM global_tags ORDER BY name`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list global tags: %w", err)
	}
	defer rows.Close()

	var tags []types.Tag
	for rows.Next() {
		var t types.Tag
		var clusterID sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &clusterID, &t.DocumentCount, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if clusterID.Valid {
			t.GroupID = clusterID.String
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// ListByGroup implements ITagRepository.ListByGroup
func (r *TagRepo) ListByGroup(ctx context.Context, groupID string) ([]types.Tag, error) {
	query := `SELECT id, name, cluster_id, document_count, created_at, updated_at FROM global_tags WHERE cluster_id = ? ORDER BY name`
	if r.driver == "postgres" {
		query = `SELECT id, name, cluster_id, document_count, created_at, updated_at FROM global_tags WHERE cluster_id = $1 ORDER BY name`
	}

	rows, err := r.db.QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("list global tags by group: %w", err)
	}
	defer rows.Close()

	var tags []types.Tag
	for rows.Next() {
		var t types.Tag
		var cid sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &cid, &t.DocumentCount, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if cid.Valid {
			t.GroupID = cid.String
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// IncrementDocumentCount implements ITagRepository.IncrementDocumentCount
func (r *TagRepo) IncrementDocumentCount(ctx context.Context, tagName string) error {
	query := `UPDATE global_tags SET document_count = document_count + 1, updated_at = ? WHERE name = ?`
	if r.driver == "postgres" {
		query = `UPDATE global_tags SET document_count = document_count + 1, updated_at = $1 WHERE name = $2`
	}

	_, err := r.db.ExecContext(ctx, query, time.Now(), tagName)
	if err != nil {
		return fmt.Errorf("increment document count: %w", err)
	}
	return nil
}

// DeleteAll implements ITagRepository.DeleteAll
func (r *TagRepo) DeleteAll(ctx context.Context) error {
	query := `DELETE FROM global_tags`
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("delete all global tags: %w", err)
	}
	return nil
}

// GetCount implements ITagRepository.GetCount
func (r *TagRepo) GetCount(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM global_tags`

	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count global tags: %w", err)
	}
	return count, nil
}

var _ storage.ITagRepository = (*TagRepo)(nil)

// TagGroupRepo implements storage.ITagGroupRepository
type TagGroupRepo struct {
	db     sqlDB
	driver string
	cache  storage.ICache
}

// NewTagGroupRepo creates a new tag group repository
func NewTagGroupRepo(db sqlDB, driver string, cache storage.ICache) *TagGroupRepo {
	return &TagGroupRepo{
		db:     db,
		driver: driver,
		cache:  cache,
	}
}

// Create implements ITagGroupRepository.Create
func (r *TagGroupRepo) Create(ctx context.Context, group *types.TagGroup) error {
	if group.ID == "" {
		group.ID = uuid.New().String()
	}
	now := time.Now()
	group.CreatedAt = now
	group.UpdatedAt = now

	query := `INSERT INTO tag_clusters (id, name, description, tags, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`
	if r.driver == "postgres" {
		query = `INSERT INTO tag_clusters (id, name, description, tags, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`
	}

	_, err := r.db.ExecContext(ctx, query, group.ID, group.Name, group.Description, group.Tags, group.CreatedAt, group.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create tag group: %w", err)
	}
	return nil
}

// GetByID implements ITagGroupRepository.GetByID
func (r *TagGroupRepo) GetByID(ctx context.Context, id string) (*types.TagGroup, error) {
	query := `SELECT id, name, description, tags, created_at, updated_at FROM tag_clusters WHERE id = ?`
	if r.driver == "postgres" {
		query = `SELECT id, name, description, tags, created_at, updated_at FROM tag_clusters WHERE id = $1`
	}

	var c types.TagGroup
	var tagsStr string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.Name, &c.Description, &tagsStr, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get tag group by id: %w", err)
	}

	c.Tags = parseStringArray(tagsStr)
	return &c, nil
}

// List implements ITagGroupRepository.List
func (r *TagGroupRepo) List(ctx context.Context) ([]types.TagGroup, error) {
	query := `SELECT id, name, description, tags, created_at, updated_at FROM tag_clusters ORDER BY name`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list tag groups: %w", err)
	}
	defer rows.Close()

	var groups []types.TagGroup
	for rows.Next() {
		var g types.TagGroup
		var tagsStr string
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &tagsStr, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		g.Tags = parseStringArray(tagsStr)
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// DeleteAll implements ITagGroupRepository.DeleteAll
func (r *TagGroupRepo) DeleteAll(ctx context.Context) error {
	query := `DELETE FROM tag_clusters`
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("delete all tag groups: %w", err)
	}
	return nil
}

// GetCount implements ITagGroupRepository.GetCount
func (r *TagGroupRepo) GetCount(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM tag_clusters`

	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count tag groups: %w", err)
	}
	return count, nil
}

var _ storage.ITagGroupRepository = (*TagGroupRepo)(nil)

// TagGroupRefreshLogRepo implements storage.ITagGroupRefreshLogRepository
type TagGroupRefreshLogRepo struct {
	db     sqlDB
	driver string
}

// NewTagGroupRefreshLogRepo creates a new tag group refresh log repository
func NewTagGroupRefreshLogRepo(db sqlDB, driver string) *TagGroupRefreshLogRepo {
	return &TagGroupRefreshLogRepo{
		db:     db,
		driver: driver,
	}
}

// Create implements ITagGroupRefreshLogRepository.Create
func (r *TagGroupRefreshLogRepo) Create(ctx context.Context, tagCountBefore, tagCountAfter, groupCount int, durationMs int64) error {
	query := `INSERT INTO cluster_refresh_log (tag_count_before, tag_count_after, cluster_count, duration_ms, created_at) VALUES (?, ?, ?, ?, ?)`
	if r.driver == "postgres" {
		query = `INSERT INTO cluster_refresh_log (tag_count_before, tag_count_after, cluster_count, duration_ms, created_at) VALUES ($1, $2, $3, $4, $5)`
	}

	_, err := r.db.ExecContext(ctx, query, tagCountBefore, tagCountAfter, groupCount, durationMs, time.Now())
	if err != nil {
		return fmt.Errorf("create tag group refresh log: %w", err)
	}
	return nil
}

// GetLastRefresh implements ITagGroupRefreshLogRepository.GetLastRefresh
func (r *TagGroupRefreshLogRepo) GetLastRefresh(ctx context.Context) (*storage.TagGroupRefreshLog, error) {
	query := `SELECT id, tag_count_before, tag_count_after, cluster_count, duration_ms, created_at FROM cluster_refresh_log ORDER BY created_at DESC LIMIT 1`

	var log storage.TagGroupRefreshLog
	var createdAt interface{}
	err := r.db.QueryRowContext(ctx, query).Scan(
		&log.ID, &log.TagCountBefore, &log.TagCountAfter, &log.GroupCount, &log.DurationMs, &createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get last refresh: %w", err)
	}

	log.CreatedAt = createdAt
	return &log, nil
}

var _ storage.ITagGroupRefreshLogRepository = (*TagGroupRefreshLogRepo)(nil)

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
	Documents           storage.IDocumentRepository
	Summaries           storage.ISummaryRepository
	Tags          storage.ITagRepository
	TagGroups         storage.ITagGroupRepository
	TagGroupRefreshLogs storage.ITagGroupRefreshLogRepository
}

// NewUnitOfWork creates a new unit of work
func NewUnitOfWork(db sqlDB, driver string, cache storage.ICache) *UnitOfWork {
	return &UnitOfWork{
		Documents:           NewDocumentRepo(db, driver, cache),
		Summaries:           NewSummaryRepo(db, driver, cache),
		Tags:          NewTagRepo(db, driver, cache),
		TagGroups:         NewTagGroupRepo(db, driver, cache),
		TagGroupRefreshLogs: NewTagGroupRefreshLogRepo(db, driver),
	}
}
