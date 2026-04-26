package document

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/db/shared"
	"github.com/tiersum/tiersum/pkg/types"
)

// TagRepo implements storage.ITagRepository
type TagRepo struct {
	db     shared.SQLDB
	driver string
}

// NewTagRepo creates a new catalog tag repository
func NewTagRepo(db shared.SQLDB, driver string) *TagRepo {
	return &TagRepo{
		db:     db,
		driver: driver,
	}
}

// Create implements ITagRepository.Create
func (r *TagRepo) Create(ctx context.Context, tag *types.Tag) error {
	ctx, span := shared.WithRepoSpan(ctx, "TagRepo.Create")
	if span != nil {
		defer span.End()
	}
	if tag.ID == "" {
		tag.ID = uuid.New().String()
	}
	now := time.Now()
	tag.CreatedAt = now
	tag.UpdatedAt = now

	v6 := shared.PlaceholdersCSV(r.driver, 6)
	ph7 := shared.Placeholder(r.driver, 7, "")
	query := fmt.Sprintf(`INSERT INTO tags (id, name, topic_id, document_count, created_at, updated_at) VALUES (%s) 
			  ON CONFLICT(name) DO UPDATE SET updated_at = %s`, v6, ph7)

	_, err := r.db.ExecContext(ctx, query, tag.ID, tag.Name, tag.TopicID, tag.DocumentCount, tag.CreatedAt, tag.UpdatedAt, tag.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create catalog tag: %w", err)
	}
	return nil
}

// GetByName implements ITagRepository.GetByName
func (r *TagRepo) GetByName(ctx context.Context, name string) (*types.Tag, error) {
	ctx, span := shared.WithRepoSpan(ctx, "TagRepo.GetByName")
	if span != nil {
		defer span.End()
	}
	ph := shared.Placeholder(r.driver, 1, "")
	query := fmt.Sprintf(`SELECT id, name, topic_id, document_count, created_at, updated_at FROM tags WHERE name = %s`, ph)

	var t types.Tag
	var topicID sql.NullString
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&t.ID, &t.Name, &topicID, &t.DocumentCount, &t.CreatedAt, &t.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get catalog tag by name: %w", err)
	}

	if topicID.Valid {
		t.TopicID = topicID.String
	}
	return &t, nil
}

// List implements ITagRepository.List
func (r *TagRepo) List(ctx context.Context) ([]types.Tag, error) {
	ctx, span := shared.WithRepoSpan(ctx, "TagRepo.List")
	if span != nil {
		defer span.End()
	}
	query := `SELECT id, name, topic_id, document_count, created_at, updated_at FROM tags ORDER BY name`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list catalog tags: %w", err)
	}
	defer rows.Close()

	var tags []types.Tag
	for rows.Next() {
		var t types.Tag
		var topicRef sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &topicRef, &t.DocumentCount, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if topicRef.Valid {
			t.TopicID = topicRef.String
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// ListByTopic implements ITagRepository.ListByTopic
func (r *TagRepo) ListByTopic(ctx context.Context, topicID string) ([]types.Tag, error) {
	ctx, span := shared.WithRepoSpan(ctx, "TagRepo.ListByTopic")
	if span != nil {
		defer span.End()
	}
	ph := shared.Placeholder(r.driver, 1, "")
	query := fmt.Sprintf(`SELECT id, name, topic_id, document_count, created_at, updated_at FROM tags WHERE topic_id = %s ORDER BY name`, ph)

	rows, err := r.db.QueryContext(ctx, query, topicID)
	if err != nil {
		return nil, fmt.Errorf("list catalog tags by topic: %w", err)
	}
	defer rows.Close()

	var tags []types.Tag
	for rows.Next() {
		var t types.Tag
		var tid sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &tid, &t.DocumentCount, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if tid.Valid {
			t.TopicID = tid.String
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// ListByTopicIDs implements ITagRepository.ListByTopicIDs
func (r *TagRepo) ListByTopicIDs(ctx context.Context, topicIDs []string, limit int) ([]types.Tag, error) {
	ctx, span := shared.WithRepoSpan(ctx, "TagRepo.ListByTopicIDs")
	if span != nil {
		defer span.End()
	}
	if len(topicIDs) == 0 {
		return []types.Tag{}, nil
	}
	if limit <= 0 {
		limit = 100
	}
	placeholders, args := shared.BuildInPlaceholders(r.driver, topicIDs)
	limitPh := shared.Placeholder(r.driver, len(topicIDs)+1, "")
	query := fmt.Sprintf(`SELECT id, name, topic_id, document_count, created_at, updated_at FROM tags 
			WHERE topic_id IN (%s) ORDER BY topic_id, name LIMIT %s`, placeholders, limitPh)
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list catalog tags by topic ids: %w", err)
	}
	defer rows.Close()

	var tags []types.Tag
	for rows.Next() {
		var t types.Tag
		var tid sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &tid, &t.DocumentCount, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if tid.Valid {
			t.TopicID = tid.String
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// IncrementDocumentCount implements ITagRepository.IncrementDocumentCount
func (r *TagRepo) IncrementDocumentCount(ctx context.Context, tagName string) error {
	ctx, span := shared.WithRepoSpan(ctx, "TagRepo.IncrementDocumentCount")
	if span != nil {
		defer span.End()
	}
	ph1 := shared.Placeholder(r.driver, 1, "")
	ph2 := shared.Placeholder(r.driver, 2, "")
	query := fmt.Sprintf(`UPDATE tags SET document_count = document_count + 1, updated_at = %s WHERE name = %s`, ph1, ph2)

	_, err := r.db.ExecContext(ctx, query, time.Now(), tagName)
	if err != nil {
		return fmt.Errorf("increment document count: %w", err)
	}
	return nil
}

// DeleteAll implements ITagRepository.DeleteAll
func (r *TagRepo) DeleteAll(ctx context.Context) error {
	ctx, span := shared.WithRepoSpan(ctx, "TagRepo.DeleteAll")
	if span != nil {
		defer span.End()
	}
	query := `DELETE FROM tags`
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("delete all catalog tags: %w", err)
	}
	return nil
}

// GetCount implements ITagRepository.GetCount
func (r *TagRepo) GetCount(ctx context.Context) (int, error) {
	ctx, span := shared.WithRepoSpan(ctx, "TagRepo.GetCount")
	if span != nil {
		defer span.End()
	}
	query := `SELECT COUNT(*) FROM tags`

	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count catalog tags: %w", err)
	}
	return count, nil
}

var _ storage.ITagRepository = (*TagRepo)(nil)
