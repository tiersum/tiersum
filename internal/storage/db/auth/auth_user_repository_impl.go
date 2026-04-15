package auth

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/db/shared"
)

// AuthUserRepo implements storage.IAuthUserRepository.
type AuthUserRepo struct {
	db     shared.SQLDB
	driver string
}

func NewAuthUserRepo(db shared.SQLDB, driver string) *AuthUserRepo {
	return &AuthUserRepo{db: db, driver: driver}
}

func (r *AuthUserRepo) Create(ctx context.Context, u *storage.AuthUser) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	u.CreatedAt = now
	u.UpdatedAt = now
	q := `INSERT INTO users (id, username, role, access_token_hash, token_expiry_mode, max_devices, token_valid_until, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	args := []interface{}{u.ID, u.Username, u.Role, u.AccessTokenHash, u.TokenExpiryMode, u.MaxDevices, u.TokenValidUntil, u.CreatedAt, u.UpdatedAt}
	if r.driver == "postgres" {
		q = `INSERT INTO users (id, username, role, access_token_hash, token_expiry_mode, max_devices, token_valid_until, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9)`
		args = []interface{}{u.ID, u.Username, u.Role, u.AccessTokenHash, u.TokenExpiryMode, u.MaxDevices, u.TokenValidUntil, u.CreatedAt, u.UpdatedAt}
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *AuthUserRepo) scanUser(scanner shared.RowScanner) (*storage.AuthUser, error) {
	u := &storage.AuthUser{}
	var validUntil sql.NullTime
	err := scanner.Scan(&u.ID, &u.Username, &u.Role, &u.AccessTokenHash, &u.TokenExpiryMode, &u.MaxDevices, &validUntil, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if validUntil.Valid {
		t := validUntil.Time
		u.TokenValidUntil = &t
	}
	return u, nil
}

func (r *AuthUserRepo) GetByID(ctx context.Context, id string) (*storage.AuthUser, error) {
	q := `SELECT id, username, role, access_token_hash, token_expiry_mode, max_devices, token_valid_until, created_at, updated_at FROM users WHERE id = ?`
	args := []interface{}{id}
	if r.driver == "postgres" {
		q = `SELECT id::text, username, role, access_token_hash, token_expiry_mode, max_devices, token_valid_until, created_at, updated_at FROM users WHERE id = $1::uuid`
	}
	row := r.db.QueryRowContext(ctx, q, args...)
	return r.scanUser(row)
}

func (r *AuthUserRepo) GetByUsername(ctx context.Context, username string) (*storage.AuthUser, error) {
	q := `SELECT id, username, role, access_token_hash, token_expiry_mode, max_devices, token_valid_until, created_at, updated_at FROM users WHERE username = ?`
	args := []interface{}{username}
	if r.driver == "postgres" {
		q = `SELECT id::text, username, role, access_token_hash, token_expiry_mode, max_devices, token_valid_until, created_at, updated_at FROM users WHERE username = $1`
	}
	row := r.db.QueryRowContext(ctx, q, args...)
	return r.scanUser(row)
}

func (r *AuthUserRepo) GetByAccessTokenHash(ctx context.Context, hashHex string) (*storage.AuthUser, error) {
	q := `SELECT id, username, role, access_token_hash, token_expiry_mode, max_devices, token_valid_until, created_at, updated_at FROM users WHERE access_token_hash = ?`
	args := []interface{}{hashHex}
	if r.driver == "postgres" {
		q = `SELECT id::text, username, role, access_token_hash, token_expiry_mode, max_devices, token_valid_until, created_at, updated_at FROM users WHERE access_token_hash = $1`
	}
	row := r.db.QueryRowContext(ctx, q, args...)
	return r.scanUser(row)
}

func (r *AuthUserRepo) UpdateAccessToken(ctx context.Context, userID, accessTokenHashHex string, validUntil *time.Time) error {
	now := time.Now().UTC()
	q := `UPDATE users SET access_token_hash = ?, token_valid_until = ?, updated_at = ? WHERE id = ?`
	args := []interface{}{accessTokenHashHex, validUntil, now, userID}
	if r.driver == "postgres" {
		q = `UPDATE users SET access_token_hash = $1, token_valid_until = $2, updated_at = $3 WHERE id = $4::uuid`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *AuthUserRepo) UpdateTokenExpiryMode(ctx context.Context, userID, mode string) error {
	now := time.Now().UTC()
	q := `UPDATE users SET token_expiry_mode = ?, updated_at = ? WHERE id = ?`
	args := []interface{}{mode, now, userID}
	if r.driver == "postgres" {
		q = `UPDATE users SET token_expiry_mode = $1, updated_at = $2 WHERE id = $3::uuid`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *AuthUserRepo) UpdateTokenValidUntil(ctx context.Context, userID string, validUntil *time.Time) error {
	now := time.Now().UTC()
	q := `UPDATE users SET token_valid_until = ?, updated_at = ? WHERE id = ?`
	args := []interface{}{validUntil, now, userID}
	if r.driver == "postgres" {
		q = `UPDATE users SET token_valid_until = $1, updated_at = $2 WHERE id = $3::uuid`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *AuthUserRepo) List(ctx context.Context) ([]storage.AuthUser, error) {
	q := `SELECT id, username, role, access_token_hash, token_expiry_mode, max_devices, token_valid_until, created_at, updated_at FROM users ORDER BY username`
	if r.driver == "postgres" {
		q = `SELECT id::text, username, role, access_token_hash, token_expiry_mode, max_devices, token_valid_until, created_at, updated_at FROM users ORDER BY username`
	}
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []storage.AuthUser
	for rows.Next() {
		u, err := r.scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}
