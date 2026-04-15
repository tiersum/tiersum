package auth

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/db/shared"
)

// PasskeyCredentialRepo implements storage.IPasskeyCredentialRepository.
type PasskeyCredentialRepo struct {
	db     shared.SQLDB
	driver string
}

func NewPasskeyCredentialRepo(db shared.SQLDB, driver string) *PasskeyCredentialRepo {
	return &PasskeyCredentialRepo{db: db, driver: driver}
}

func (r *PasskeyCredentialRepo) Create(ctx context.Context, c *storage.PasskeyCredential) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	q := `INSERT INTO passkey_credentials (id, user_id, credential_id_b64, public_key_b64, sign_count, device_name, last_used_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	args := []any{c.ID, c.UserID, c.CredentialIDB64, c.PublicKeyB64, c.SignCount, c.DeviceName, c.LastUsedAt, c.CreatedAt}
	if r.driver == "postgres" {
		q = `INSERT INTO passkey_credentials (id, user_id, credential_id_b64, public_key_b64, sign_count, device_name, last_used_at, created_at)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *PasskeyCredentialRepo) scan(scanner shared.RowScanner) (*storage.PasskeyCredential, error) {
	var c storage.PasskeyCredential
	if err := scanner.Scan(&c.ID, &c.UserID, &c.CredentialIDB64, &c.PublicKeyB64, &c.SignCount, &c.DeviceName, &c.LastUsedAt, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *PasskeyCredentialRepo) ListByUser(ctx context.Context, userID string) ([]storage.PasskeyCredential, error) {
	q := `SELECT id, user_id, credential_id_b64, public_key_b64, sign_count, device_name, last_used_at, created_at
		FROM passkey_credentials WHERE user_id = ? ORDER BY created_at DESC`
	args := []any{userID}
	if r.driver == "postgres" {
		q = `SELECT id::text, user_id::text, credential_id_b64, public_key_b64, sign_count, device_name, last_used_at, created_at
			FROM passkey_credentials WHERE user_id = $1::uuid ORDER BY created_at DESC`
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]storage.PasskeyCredential, 0)
	for rows.Next() {
		c, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (r *PasskeyCredentialRepo) GetByID(ctx context.Context, id string) (*storage.PasskeyCredential, error) {
	q := `SELECT id, user_id, credential_id_b64, public_key_b64, sign_count, device_name, last_used_at, created_at
		FROM passkey_credentials WHERE id = ?`
	args := []any{id}
	if r.driver == "postgres" {
		q = `SELECT id::text, user_id::text, credential_id_b64, public_key_b64, sign_count, device_name, last_used_at, created_at
			FROM passkey_credentials WHERE id = $1::uuid`
	}
	row := r.db.QueryRowContext(ctx, q, args...)
	return r.scan(row)
}

func (r *PasskeyCredentialRepo) GetByCredentialID(ctx context.Context, credentialIDB64 string) (*storage.PasskeyCredential, error) {
	q := `SELECT id, user_id, credential_id_b64, public_key_b64, sign_count, device_name, last_used_at, created_at
		FROM passkey_credentials WHERE credential_id_b64 = ?`
	args := []any{credentialIDB64}
	if r.driver == "postgres" {
		q = `SELECT id::text, user_id::text, credential_id_b64, public_key_b64, sign_count, device_name, last_used_at, created_at
			FROM passkey_credentials WHERE credential_id_b64 = $1`
	}
	row := r.db.QueryRowContext(ctx, q, args...)
	return r.scan(row)
}

func (r *PasskeyCredentialRepo) UpdateSignCountAndLastUsed(ctx context.Context, id string, signCount int64, at time.Time) error {
	q := `UPDATE passkey_credentials SET sign_count = ?, last_used_at = ? WHERE id = ?`
	args := []any{signCount, at, id}
	if r.driver == "postgres" {
		q = `UPDATE passkey_credentials SET sign_count = $1, last_used_at = $2 WHERE id = $3::uuid`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *PasskeyCredentialRepo) Delete(ctx context.Context, id string) error {
	q := `DELETE FROM passkey_credentials WHERE id = ?`
	args := []any{id}
	if r.driver == "postgres" {
		q = `DELETE FROM passkey_credentials WHERE id = $1::uuid`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

var _ storage.IPasskeyCredentialRepository = (*PasskeyCredentialRepo)(nil)

