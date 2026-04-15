package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/tiersum/tiersum/internal/storage"
)

// APIKeyRepo implements storage.IAPIKeyRepository.
type APIKeyRepo struct {
	db     sqlDB
	driver string
}

func NewAPIKeyRepo(db sqlDB, driver string) *APIKeyRepo {
	return &APIKeyRepo{db: db, driver: driver}
}

func (r *APIKeyRepo) Create(ctx context.Context, k *storage.APIKey) error {
	if k.ID == "" {
		k.ID = uuid.New().String()
	}
	k.CreatedAt = time.Now().UTC()
	var createdBy interface{}
	if k.CreatedByUserID != nil && *k.CreatedByUserID != "" {
		createdBy = *k.CreatedByUserID
	} else {
		createdBy = nil
	}
	q := `INSERT INTO api_keys (id, name, scope, key_hash, revoked_at, expires_at, created_by_user_id, last_used_at, last_used_ip, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	args := []interface{}{k.ID, k.Name, k.Scope, k.KeyHash, k.RevokedAt, k.ExpiresAt, createdBy, k.LastUsedAt, k.LastUsedIP, k.CreatedAt}
	if r.driver == "postgres" {
		q = `INSERT INTO api_keys (id, name, scope, key_hash, revoked_at, expires_at, created_by_user_id, last_used_at, last_used_ip, created_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
		args = []interface{}{k.ID, k.Name, k.Scope, k.KeyHash, k.RevokedAt, k.ExpiresAt, createdBy, k.LastUsedAt, k.LastUsedIP, k.CreatedAt}
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *APIKeyRepo) scanKey(scanner rowScanner) (*storage.APIKey, error) {
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

func (r *APIKeyRepo) GetByID(ctx context.Context, id string) (*storage.APIKey, error) {
	q := `SELECT id, name, scope, key_hash, revoked_at, expires_at, created_by_user_id, last_used_at, last_used_ip, created_at FROM api_keys WHERE id = ?`
	args := []interface{}{id}
	if r.driver == "postgres" {
		q = `SELECT id::text, name, scope, key_hash, revoked_at, expires_at, created_by_user_id::text, last_used_at, last_used_ip, created_at FROM api_keys WHERE id = $1::uuid`
	}
	row := r.db.QueryRowContext(ctx, q, args...)
	return r.scanKey(row)
}

func (r *APIKeyRepo) GetByKeyHash(ctx context.Context, keyHashHex string) (*storage.APIKey, error) {
	q := `SELECT id, name, scope, key_hash, revoked_at, expires_at, created_by_user_id, last_used_at, last_used_ip, created_at FROM api_keys WHERE key_hash = ?`
	args := []interface{}{keyHashHex}
	if r.driver == "postgres" {
		q = `SELECT id::text, name, scope, key_hash, revoked_at, expires_at, created_by_user_id::text, last_used_at, last_used_ip, created_at FROM api_keys WHERE key_hash = $1`
	}
	row := r.db.QueryRowContext(ctx, q, args...)
	return r.scanKey(row)
}

func (r *APIKeyRepo) GetActiveByKeyHash(ctx context.Context, keyHashHex string) (*storage.APIKey, error) {
	now := time.Now().UTC()
	q := `SELECT id, name, scope, key_hash, revoked_at, expires_at, created_by_user_id, last_used_at, last_used_ip, created_at FROM api_keys
		WHERE key_hash = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)`
	args := []interface{}{keyHashHex, now}
	if r.driver == "postgres" {
		q = `SELECT id::text, name, scope, key_hash, revoked_at, expires_at, created_by_user_id::text, last_used_at, last_used_ip, created_at FROM api_keys
			WHERE key_hash = $1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > $2)`
	}
	row := r.db.QueryRowContext(ctx, q, args...)
	return r.scanKey(row)
}

func (r *APIKeyRepo) List(ctx context.Context) ([]storage.APIKey, error) {
	q := `SELECT id, name, scope, key_hash, revoked_at, expires_at, created_by_user_id, last_used_at, last_used_ip, created_at FROM api_keys ORDER BY created_at DESC`
	if r.driver == "postgres" {
		q = `SELECT id::text, name, scope, key_hash, revoked_at, expires_at, created_by_user_id::text, last_used_at, last_used_ip, created_at FROM api_keys ORDER BY created_at DESC`
	}
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []storage.APIKey
	for rows.Next() {
		k, err := r.scanKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *k)
	}
	return out, rows.Err()
}

func (r *APIKeyRepo) Revoke(ctx context.Context, id string) error {
	now := time.Now().UTC()
	q := `UPDATE api_keys SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`
	args := []interface{}{now, id}
	if r.driver == "postgres" {
		q = `UPDATE api_keys SET revoked_at = $1 WHERE id = $2::uuid AND revoked_at IS NULL`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *APIKeyRepo) TouchLastUsed(ctx context.Context, id, clientIP string, at time.Time) error {
	q := `UPDATE api_keys SET last_used_at = ?, last_used_ip = ? WHERE id = ?`
	args := []interface{}{at, clientIP, id}
	if r.driver == "postgres" {
		q = `UPDATE api_keys SET last_used_at = $1, last_used_ip = $2 WHERE id = $3::uuid`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}
