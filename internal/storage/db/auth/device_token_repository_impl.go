package auth

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/db/shared"
)

// DeviceTokenRepo implements storage.IDeviceTokenRepository.
type DeviceTokenRepo struct {
	db     shared.SQLDB
	driver string
}

func NewDeviceTokenRepo(db shared.SQLDB, driver string) *DeviceTokenRepo {
	return &DeviceTokenRepo{db: db, driver: driver}
}

func (r *DeviceTokenRepo) Create(ctx context.Context, t *storage.DeviceToken) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	q := `INSERT INTO device_tokens (id, user_id, token_hash, device_name, ip_prefix, user_agent_norm, last_used_at, expires_at, revoked_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	args := []any{t.ID, t.UserID, t.TokenHash, t.DeviceName, t.IPPrefix, t.UserAgentNorm, t.LastUsedAt, t.ExpiresAt, t.RevokedAt, t.CreatedAt}
	if r.driver == "postgres" {
		q = `INSERT INTO device_tokens (id, user_id, token_hash, device_name, ip_prefix, user_agent_norm, last_used_at, expires_at, revoked_at, created_at)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10)`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *DeviceTokenRepo) scan(scanner shared.RowScanner) (*storage.DeviceToken, error) {
	var t storage.DeviceToken
	if err := scanner.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.DeviceName, &t.IPPrefix, &t.UserAgentNorm, &t.LastUsedAt, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *DeviceTokenRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*storage.DeviceToken, error) {
	q := `SELECT id, user_id, token_hash, device_name, ip_prefix, user_agent_norm, last_used_at, expires_at, revoked_at, created_at
		FROM device_tokens WHERE token_hash = ?`
	args := []any{tokenHash}
	if r.driver == "postgres" {
		q = `SELECT id::text, user_id::text, token_hash, device_name, ip_prefix, user_agent_norm, last_used_at, expires_at, revoked_at, created_at
			FROM device_tokens WHERE token_hash = $1`
	}
	row := r.db.QueryRowContext(ctx, q, args...)
	return r.scan(row)
}

func (r *DeviceTokenRepo) ListByUser(ctx context.Context, userID string) ([]storage.DeviceToken, error) {
	q := `SELECT id, user_id, token_hash, device_name, ip_prefix, user_agent_norm, last_used_at, expires_at, revoked_at, created_at
		FROM device_tokens WHERE user_id = ? ORDER BY created_at DESC`
	args := []any{userID}
	if r.driver == "postgres" {
		q = `SELECT id::text, user_id::text, token_hash, device_name, ip_prefix, user_agent_norm, last_used_at, expires_at, revoked_at, created_at
			FROM device_tokens WHERE user_id = $1::uuid ORDER BY created_at DESC`
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]storage.DeviceToken, 0)
	for rows.Next() {
		t, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (r *DeviceTokenRepo) TouchUse(ctx context.Context, id string, at time.Time) error {
	q := `UPDATE device_tokens SET last_used_at = ? WHERE id = ?`
	args := []any{at, id}
	if r.driver == "postgres" {
		q = `UPDATE device_tokens SET last_used_at = $1 WHERE id = $2::uuid`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *DeviceTokenRepo) Revoke(ctx context.Context, id string, at time.Time) error {
	q := `UPDATE device_tokens SET revoked_at = ? WHERE id = ?`
	args := []any{at, id}
	if r.driver == "postgres" {
		q = `UPDATE device_tokens SET revoked_at = $1 WHERE id = $2::uuid`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *DeviceTokenRepo) RevokeAllForUser(ctx context.Context, userID string, at time.Time) error {
	q := `UPDATE device_tokens SET revoked_at = ? WHERE user_id = ?`
	args := []any{at, userID}
	if r.driver == "postgres" {
		q = `UPDATE device_tokens SET revoked_at = $1 WHERE user_id = $2::uuid`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

var _ storage.IDeviceTokenRepository = (*DeviceTokenRepo)(nil)

