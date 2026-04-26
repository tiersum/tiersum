package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/db/shared"
)

// SystemAuthStateRepo implements storage.ISystemAuthStateRepository.
type SystemAuthStateRepo struct {
	db     shared.SQLDB
	driver string
}

func NewSystemAuthStateRepo(db shared.SQLDB, driver string) *SystemAuthStateRepo {
	return &SystemAuthStateRepo{db: db, driver: driver}
}

func (r *SystemAuthStateRepo) Get(ctx context.Context) (*storage.SystemAuthState, error) {
	ctx, span := shared.WithRepoSpan(ctx, "SystemAuthStateRepo.Get")
	if span != nil { defer span.End() }
	const q = `SELECT initialized_at FROM system_state WHERE id = 1`
	var t sql.NullTime
	err := r.db.QueryRowContext(ctx, q).Scan(&t)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Missing singleton row (empty or legacy DB): treat as uninitialized, not a hard failure.
			if seedErr := r.ensureSingletonRow(ctx); seedErr != nil {
				shared.SetSpanStatus(span, seedErr)
				return nil, seedErr
			}
			shared.SetSpanStatus(span, nil)
			return &storage.SystemAuthState{}, nil
		}
		shared.SetSpanStatus(span, err)
		return nil, err
	}
	st := &storage.SystemAuthState{}
	if t.Valid {
		st.InitializedAt = &t.Time
	}
	shared.SetSpanStatus(span, nil)
	return st, nil
}

// ensureSingletonRow inserts the default system_state row if absent (id=1).
func (r *SystemAuthStateRepo) ensureSingletonRow(ctx context.Context) error {
	var q string
	if shared.DriverIsPostgres(r.driver) {
		q = `INSERT INTO system_state (id, initialized_at) VALUES (1, NULL) ON CONFLICT (id) DO NOTHING`
	} else {
		q = `INSERT OR IGNORE INTO system_state (id, initialized_at) VALUES (1, NULL)`
	}
	_, err := r.db.ExecContext(ctx, q)
	return err
}

func (r *SystemAuthStateRepo) MarkInitialized(ctx context.Context) error {
	ctx, span := shared.WithRepoSpan(ctx, "SystemAuthStateRepo.MarkInitialized")
	if span != nil { defer span.End() }
	if err := r.ensureSingletonRow(ctx); err != nil {
		shared.SetSpanStatus(span, err)
		return err
	}
	now := time.Now().UTC()
	ph := shared.Placeholder(r.driver, 1, "")
	q := fmt.Sprintf(`UPDATE system_state SET initialized_at = %s WHERE id = 1 AND initialized_at IS NULL`, ph)
	res, err := r.db.ExecContext(ctx, q, now)
	if err != nil {
		shared.SetSpanStatus(span, err)
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		err := fmt.Errorf("system already initialized or state row missing")
		shared.SetSpanStatus(span, err)
		return err
	}
	shared.SetSpanStatus(span, nil)
	return nil
}
