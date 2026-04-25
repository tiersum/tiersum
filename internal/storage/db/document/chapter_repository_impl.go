package document

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/db/shared"
	"github.com/tiersum/tiersum/pkg/types"
)

// ChapterRepo implements storage.IChapterRepository.
type ChapterRepo struct {
	db     shared.SQLDB
	driver string
	cache  storage.ICache
}

// NewChapterRepo creates a new chapter repository.
func NewChapterRepo(db shared.SQLDB, driver string, cache storage.ICache) *ChapterRepo {
	return &ChapterRepo{
		db:     db,
		driver: driver,
		cache:  cache,
	}
}

// ReplaceByDocument deletes all chapters for document_id and inserts the given rows.
func (r *ChapterRepo) ReplaceByDocument(ctx context.Context, documentID string, chapters []types.Chapter) error {
	ctx, span := shared.WithRepoSpan(ctx, "ChapterRepo.ReplaceByDocument")
	if span != nil {
		defer span.End()
	}
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return fmt.Errorf("replace chapters: document id is required")
	}
	phDel := shared.Placeholder(r.driver, 1, "")
	del := fmt.Sprintf(`DELETE FROM chapters WHERE document_id = %s`, phDel)
	if _, err := r.db.ExecContext(ctx, del, documentID); err != nil {
		return fmt.Errorf("delete chapters by document: %w", err)
	}
	if len(chapters) == 0 {
		return nil
	}
	for i := range chapters {
		ch := &chapters[i]
		if ch.ID == "" {
			ch.ID = uuid.New().String()
		}
		now := time.Now()
		if ch.CreatedAt.IsZero() {
			ch.CreatedAt = now
		}
		ch.UpdatedAt = now
		if ch.DocumentID == "" {
			ch.DocumentID = documentID
		}
		vals := shared.PlaceholdersCSV(r.driver, 8)
		ins := fmt.Sprintf(`INSERT INTO chapters (id, document_id, path, title, summary, content, created_at, updated_at) VALUES (%s)`, vals)
		if _, err := r.db.ExecContext(ctx, ins, ch.ID, ch.DocumentID, ch.Path, ch.Title, ch.Summary, ch.Content, ch.CreatedAt, ch.UpdatedAt); err != nil {
			return fmt.Errorf("insert chapter: %w", err)
		}
	}
	if r.cache != nil {
		r.cache.Set("chapters:"+documentID, nil)
	}
	return nil
}

func (r *ChapterRepo) ListByDocument(ctx context.Context, documentID string) ([]types.Chapter, error) {
	ctx, span := shared.WithRepoSpan(ctx, "ChapterRepo.ListByDocument")
	if span != nil {
		defer span.End()
	}
	cacheKey := "chapters:" + documentID
	if r.cache != nil {
		if cached, ok := r.cache.Get(cacheKey); ok {
			if cached == nil {
				// Cache invalidation marker: treat as miss.
			} else if out, ok := cached.([]types.Chapter); ok {
				return out, nil
			}
		}
	}
	ph := shared.Placeholder(r.driver, 1, "")
	q := fmt.Sprintf(`SELECT id, document_id, path, title, summary, content, created_at, updated_at FROM chapters WHERE document_id = %s ORDER BY created_at`, ph)
	rows, err := r.db.QueryContext(ctx, q, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.Chapter
	for rows.Next() {
		var c types.Chapter
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.Path, &c.Title, &c.Summary, &c.Content, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if r.cache != nil {
		r.cache.Set(cacheKey, out)
	}
	return out, nil
}

func (r *ChapterRepo) ListByDocumentIDs(ctx context.Context, documentIDs []string) ([]types.Chapter, error) {
	ctx, span := shared.WithRepoSpan(ctx, "ChapterRepo.ListByDocumentIDs")
	if span != nil {
		defer span.End()
	}
	if len(documentIDs) == 0 {
		return nil, nil
	}
	placeholders, args := shared.BuildInPlaceholders(r.driver, documentIDs)
	q := fmt.Sprintf(`SELECT id, document_id, path, title, summary, content, created_at, updated_at FROM chapters WHERE document_id IN (%s) ORDER BY document_id, created_at`, placeholders)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.Chapter
	for rows.Next() {
		var c types.Chapter
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.Path, &c.Title, &c.Summary, &c.Content, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *ChapterRepo) ListByIDs(ctx context.Context, chapterIDs []string) ([]types.Chapter, error) {
	ctx, span := shared.WithRepoSpan(ctx, "ChapterRepo.ListByIDs")
	if span != nil {
		defer span.End()
	}
	if len(chapterIDs) == 0 {
		return nil, nil
	}
	placeholders, args := shared.BuildInPlaceholders(r.driver, chapterIDs)
	q := fmt.Sprintf(`SELECT id, document_id, path, title, summary, content, created_at, updated_at FROM chapters WHERE id IN (%s)`, placeholders)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.Chapter
	for rows.Next() {
		var c types.Chapter
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.Path, &c.Title, &c.Summary, &c.Content, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

var _ storage.IChapterRepository = (*ChapterRepo)(nil)
