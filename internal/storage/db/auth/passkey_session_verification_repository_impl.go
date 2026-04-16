package auth

import (
	"context"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/db/shared"
)

// PasskeySessionVerificationRepo implements storage.IPasskeySessionVerificationRepository.
type PasskeySessionVerificationRepo struct {
	db     shared.SQLDB
	driver string
}

func NewPasskeySessionVerificationRepo(db shared.SQLDB, driver string) *PasskeySessionVerificationRepo {
	return &PasskeySessionVerificationRepo{db: db, driver: driver}
}

func (r *PasskeySessionVerificationRepo) Put(ctx context.Context, v *storage.PasskeySessionVerification) error {
	q := `INSERT INTO passkey_session_verifications (session_id, user_id, verified_at, expires_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET user_id=excluded.user_id, verified_at=excluded.verified_at, expires_at=excluded.expires_at`
	args := []any{v.SessionID, v.UserID, v.VerifiedAt, v.ExpiresAt}
	if r.driver == "postgres" {
		q = `INSERT INTO passkey_session_verifications (session_id, user_id, verified_at, expires_at)
			VALUES ($1, $2::uuid, $3, $4)
			ON CONFLICT(session_id) DO UPDATE SET user_id=excluded.user_id, verified_at=excluded.verified_at, expires_at=excluded.expires_at`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *PasskeySessionVerificationRepo) GetBySessionID(ctx context.Context, sessionID string) (*storage.PasskeySessionVerification, error) {
	q := `SELECT session_id, user_id, verified_at, expires_at FROM passkey_session_verifications WHERE session_id = ?`
	args := []any{sessionID}
	if r.driver == "postgres" {
		q = `SELECT session_id, user_id::text, verified_at, expires_at FROM passkey_session_verifications WHERE session_id = $1`
	}
	row := r.db.QueryRowContext(ctx, q, args...)
	var v storage.PasskeySessionVerification
	if err := row.Scan(&v.SessionID, &v.UserID, &v.VerifiedAt, &v.ExpiresAt); err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *PasskeySessionVerificationRepo) DeleteBySessionID(ctx context.Context, sessionID string) error {
	q := `DELETE FROM passkey_session_verifications WHERE session_id = ?`
	args := []any{sessionID}
	if r.driver == "postgres" {
		q = `DELETE FROM passkey_session_verifications WHERE session_id = $1`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

var _ storage.IPasskeySessionVerificationRepository = (*PasskeySessionVerificationRepo)(nil)

