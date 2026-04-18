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

// TopicRepo implements storage.ITopicRepository
type TopicRepo struct {
	db     shared.SQLDB
	driver string
}

// NewTopicRepo creates a new topic repository
func NewTopicRepo(db shared.SQLDB, driver string) *TopicRepo {
	return &TopicRepo{
		db:     db,
		driver: driver,
	}
}

// Create implements ITopicRepository.Create
func (r *TopicRepo) Create(ctx context.Context, topic *types.Topic) error {
	if topic.ID == "" {
		topic.ID = uuid.New().String()
	}
	now := time.Now()
	topic.CreatedAt = now
	topic.UpdatedAt = now

	vals := shared.PlaceholdersCSV(r.driver, 6)
	query := fmt.Sprintf(`INSERT INTO topics (id, name, description, tag_names, created_at, updated_at) VALUES (%s)`, vals)

	_, err := r.db.ExecContext(ctx, query, topic.ID, topic.Name, topic.Description, shared.FormatStringArray(topic.TagNames), topic.CreatedAt, topic.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create topic: %w", err)
	}
	return nil
}

// GetByID implements ITopicRepository.GetByID
func (r *TopicRepo) GetByID(ctx context.Context, id string) (*types.Topic, error) {
	ph := shared.Placeholder(r.driver, 1, "")
	query := fmt.Sprintf(`SELECT id, name, description, tag_names, created_at, updated_at FROM topics WHERE id = %s`, ph)

	var c types.Topic
	var tagsStr string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.Name, &c.Description, &tagsStr, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get topic by id: %w", err)
	}

	c.TagNames = shared.ParseStringArray(tagsStr)
	return &c, nil
}

// List implements ITopicRepository.List
func (r *TopicRepo) List(ctx context.Context) ([]types.Topic, error) {
	query := `SELECT id, name, description, tag_names, created_at, updated_at FROM topics ORDER BY name`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	defer rows.Close()

	var topics []types.Topic
	for rows.Next() {
		var g types.Topic
		var tagsStr string
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &tagsStr, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		g.TagNames = shared.ParseStringArray(tagsStr)
		topics = append(topics, g)
	}
	return topics, rows.Err()
}

// DeleteAll implements ITopicRepository.DeleteAll
func (r *TopicRepo) DeleteAll(ctx context.Context) error {
	query := `DELETE FROM topics`
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("delete all topics: %w", err)
	}
	return nil
}

// GetCount implements ITopicRepository.GetCount
func (r *TopicRepo) GetCount(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM topics`

	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count topics: %w", err)
	}
	return count, nil
}

var _ storage.ITopicRepository = (*TopicRepo)(nil)
