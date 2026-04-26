package document

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/db/shared"
)

// DeletedDocumentRepo implements storage.IDeletedDocumentRepository.
type DeletedDocumentRepo struct {
	db     shared.SQLDB
	driver string
}

// NewDeletedDocumentRepo creates a new tombstone repository.
func NewDeletedDocumentRepo(db shared.SQLDB, driver string) *DeletedDocumentRepo {
	return &DeletedDocumentRepo{db: db, driver: driver}
}

// Insert records a deleted document ID.
func (r *DeletedDocumentRepo) Insert(ctx context.Context, documentID string) error {
	ph1 := shared.Placeholder(r.driver, 1, "")
	ph2 := shared.Placeholder(r.driver, 2, "")
	q := fmt.Sprintf(`INSERT INTO deleted_documents (id, document_id, created_at) VALUES (%s, %s, %s)`, ph1, ph2, ph1)
	_, err := r.db.ExecContext(ctx, q, uuid.New().String(), documentID, time.Now())
	return err
}

// ListSince returns tombstones created after the given time.
func (r *DeletedDocumentRepo) ListSince(ctx context.Context, since time.Time, limit int) ([]storage.DeletedDocument, error) {
	if limit <= 0 {
		limit = 1000
	}
	ph1 := shared.Placeholder(r.driver, 1, "")
	ph2 := shared.Placeholder(r.driver, 2, "")
	q := fmt.Sprintf(`SELECT id, document_id, created_at FROM deleted_documents WHERE created_at > %s ORDER BY created_at LIMIT %s`, ph1, ph2)
	rows, err := r.db.QueryContext(ctx, q, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []storage.DeletedDocument
	for rows.Next() {
		var d storage.DeletedDocument
		if err := rows.Scan(&d.ID, &d.DocumentID, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

var _ storage.IDeletedDocumentRepository = (*DeletedDocumentRepo)(nil)
