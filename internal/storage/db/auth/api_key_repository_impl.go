package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/db/shared"
)

// APIKeyRepo implements storage.IAPIKeyRepository.
type APIKeyRepo struct {
	db     shared.SQLDB
	driver string
}

func NewAPIKeyRepo(db shared.SQLDB, driver string) *APIKeyRepo {
	return &APIKeyRepo{db: db, driver: driver}
}

func (r *APIKeyRepo) Create(ctx context.Context, k *storage.APIKey) error {
	ctx, span := shared.WithRepoSpan(ctx, "APIKeyRepo.Create")
	if span != nil { defer span.End() }
	if k.ID == "" {
		k.ID = uuid.New().String()
	}
	shared.SetSpanInputID(span, k.ID)
	k.CreatedAt = time.Now().UTC()
	var createdBy interface{}
	if k.CreatedByUserID != nil && *k.CreatedByUserID != "" {
		createdBy = *k.CreatedByUserID
	} else {
		createdBy = nil
	}
	vals := shared.PlaceholdersCSVWithPGCasts(r.driver, []string{"uuid", "", "", "", "", "", "", "", "", ""})
	q := fmt.Sprintf(`INSERT INTO api_keys (id, name, scope, key_hash, revoked_at, expires_at, created_by_user_id, last_used_at, last_used_ip, created_at)
		VALUES (%s)`, vals)
	args := []interface{}{k.ID, k.Name, k.Scope, k.KeyHash, k.RevokedAt, k.ExpiresAt, createdBy, k.LastUsedAt, k.LastUsedIP, k.CreatedAt}
	_, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		shared.SetSpanStatus(span, err)
		return err
	}
	shared.SetSpanStatus(span, nil)
	return nil
}

func (r *APIKeyRepo) scanKey(scanner shared.RowScanner) (*storage.APIKey, error) {
	k := &storage.APIKey{}
	var revoked, expires, lastUsed sql.NullTime
	var createdBy sql.NullString
	var lastIP sql.NullString
	err := scanner.Scan(&k.ID, &k.Name, &k.Scope, &k.KeyHash, &revoked, &expires, &createdBy, &lastUsed, &lastIP, &k.CreatedAt)
	if err != nil {
		return nil, err
	}
	if revoked.Valid {
		t := revoked.Time
		k.RevokedAt = &t
	}
	if expires.Valid {
		t := expires.Time
		k.ExpiresAt = &t
	}
	if createdBy.Valid {
		s := createdBy.String
		k.CreatedByUserID = &s
	}
	if lastUsed.Valid {
		t := lastUsed.Time
		k.LastUsedAt = &t
	}
	if lastIP.Valid {
		k.LastUsedIP = lastIP.String
	}
	return k, nil
}

func (r *APIKeyRepo) selectKeyCols() string {
	if shared.DriverIsPostgres(r.driver) {
		return `id::text, name, scope, key_hash, revoked_at, expires_at, created_by_user_id::text, last_used_at, last_used_ip, created_at`
	}
	return `id, name, scope, key_hash, revoked_at, expires_at, created_by_user_id, last_used_at, last_used_ip, created_at`
}

func (r *APIKeyRepo) GetByID(ctx context.Context, id string) (*storage.APIKey, error) {
	ctx, span := shared.WithRepoSpan(ctx, "APIKeyRepo.GetByID")
	if span != nil { defer span.End() }
	shared.SetSpanInputID(span, id)
	cols := r.selectKeyCols()
	ph := shared.Placeholder(r.driver, 1, "uuid")
	q := fmt.Sprintf(`SELECT %s FROM api_keys WHERE id = %s`, cols, ph)
	row := r.db.QueryRowContext(ctx, q, id)
	k, err := r.scanKey(row)
	if err != nil {
		shared.SetSpanStatus(span, err)
		return nil, err
	}
	shared.SetSpanOutputID(span, k.ID)
	shared.SetSpanStatus(span, nil)
	return k, nil
}

func (r *APIKeyRepo) GetByKeyHash(ctx context.Context, keyHashHex string) (*storage.APIKey, error) {
	ctx, span := shared.WithRepoSpan(ctx, "APIKeyRepo.GetByKeyHash")
	if span != nil { defer span.End() }
	shared.SetSpanInputString(span, "key_hash", keyHashHex)
	cols := r.selectKeyCols()
	ph := shared.Placeholder(r.driver, 1, "")
	q := fmt.Sprintf(`SELECT %s FROM api_keys WHERE key_hash = %s`, cols, ph)
	row := r.db.QueryRowContext(ctx, q, keyHashHex)
	k, err := r.scanKey(row)
	if err != nil {
		shared.SetSpanStatus(span, err)
		return nil, err
	}
	shared.SetSpanOutputID(span, k.ID)
	shared.SetSpanStatus(span, nil)
	return k, nil
}

func (r *APIKeyRepo) GetActiveByKeyHash(ctx context.Context, keyHashHex string) (*storage.APIKey, error) {
	ctx, span := shared.WithRepoSpan(ctx, "APIKeyRepo.GetActiveByKeyHash")
	if span != nil { defer span.End() }
	shared.SetSpanInputString(span, "key_hash", keyHashHex)
	now := time.Now().UTC()
	cols := r.selectKeyCols()
	ph1 := shared.Placeholder(r.driver, 1, "")
	ph2 := shared.Placeholder(r.driver, 2, "")
	q := fmt.Sprintf(`SELECT %s FROM api_keys
		WHERE key_hash = %s AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > %s)`, cols, ph1, ph2)
	row := r.db.QueryRowContext(ctx, q, keyHashHex, now)
	k, err := r.scanKey(row)
	if err != nil {
		shared.SetSpanStatus(span, err)
		return nil, err
	}
	shared.SetSpanOutputID(span, k.ID)
	shared.SetSpanStatus(span, nil)
	return k, nil
}

func (r *APIKeyRepo) List(ctx context.Context) ([]storage.APIKey, error) {
	ctx, span := shared.WithRepoSpan(ctx, "APIKeyRepo.List")
	if span != nil { defer span.End() }
	q := fmt.Sprintf(`SELECT %s FROM api_keys ORDER BY created_at DESC`, r.selectKeyCols())
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		shared.SetSpanStatus(span, err)
		return nil, err
	}
	defer rows.Close()
	var out []storage.APIKey
	for rows.Next() {
		k, err := r.scanKey(rows)
		if err != nil {
			shared.SetSpanStatus(span, err)
			return nil, err
		}
		out = append(out, *k)
	}
	if err := rows.Err(); err != nil {
		shared.SetSpanStatus(span, err)
		return nil, err
	}
	shared.SetSpanOutputCount(span, len(out))
	shared.SetSpanOutputIDs(span, shared.CollectIDs(out, func(k storage.APIKey) string { return k.ID }))
	shared.SetSpanStatus(span, nil)
	return out, nil
}

func (r *APIKeyRepo) Revoke(ctx context.Context, id string) error {
	ctx, span := shared.WithRepoSpan(ctx, "APIKeyRepo.Revoke")
	if span != nil { defer span.End() }
	shared.SetSpanInputID(span, id)
	now := time.Now().UTC()
	ph1 := shared.Placeholder(r.driver, 1, "")
	ph2 := shared.Placeholder(r.driver, 2, "uuid")
	q := fmt.Sprintf(`UPDATE api_keys SET revoked_at = %s WHERE id = %s AND revoked_at IS NULL`, ph1, ph2)
	_, err := r.db.ExecContext(ctx, q, now, id)
	if err != nil {
		shared.SetSpanStatus(span, err)
		return err
	}
	shared.SetSpanStatus(span, nil)
	return nil
}

func (r *APIKeyRepo) TouchLastUsed(ctx context.Context, id, clientIP string, at time.Time) error {
	ctx, span := shared.WithRepoSpan(ctx, "APIKeyRepo.TouchLastUsed")
	if span != nil { defer span.End() }
	shared.SetSpanInputID(span, id)
	ph1 := shared.Placeholder(r.driver, 1, "")
	ph2 := shared.Placeholder(r.driver, 2, "")
	ph3 := shared.Placeholder(r.driver, 3, "uuid")
	q := fmt.Sprintf(`UPDATE api_keys SET last_used_at = %s, last_used_ip = %s WHERE id = %s`, ph1, ph2, ph3)
	_, err := r.db.ExecContext(ctx, q, at, clientIP, id)
	if err != nil {
		shared.SetSpanStatus(span, err)
		return err
	}
	shared.SetSpanStatus(span, nil)
	return nil
}
