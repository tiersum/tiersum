package auth

import (
	"context"
	"fmt"

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
	vals := shared.PlaceholdersCSVWithPGCasts(r.driver, []string{"", "uuid", "", ""})
	q := fmt.Sprintf(`INSERT INTO passkey_session_verifications (session_id, user_id, verified_at, expires_at)
		VALUES (%s)
		ON CONFLICT(session_id) DO UPDATE SET user_id=excluded.user_id, verified_at=excluded.verified_at, expires_at=excluded.expires_at`, vals)
	_, err := r.db.ExecContext(ctx, q, v.SessionID, v.UserID, v.VerifiedAt, v.ExpiresAt)
	return err
}

func (r *PasskeySessionVerificationRepo) GetBySessionID(ctx context.Context, sessionID string) (*storage.PasskeySessionVerification, error) {
	userCol := "user_id"
	if shared.DriverIsPostgres(r.driver) {
		userCol = "user_id::text"
	}
	ph := shared.Placeholder(r.driver, 1, "")
	q := fmt.Sprintf(`SELECT session_id, %s, verified_at, expires_at FROM passkey_session_verifications WHERE session_id = %s`, userCol, ph)
	row := r.db.QueryRowContext(ctx, q, sessionID)
	var v storage.PasskeySessionVerification
	if err := row.Scan(&v.SessionID, &v.UserID, &v.VerifiedAt, &v.ExpiresAt); err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *PasskeySessionVerificationRepo) DeleteBySessionID(ctx context.Context, sessionID string) error {
	ph := shared.Placeholder(r.driver, 1, "")
	q := fmt.Sprintf(`DELETE FROM passkey_session_verifications WHERE session_id = %s`, ph)
	_, err := r.db.ExecContext(ctx, q, sessionID)
	return err
}

var _ storage.IPasskeySessionVerificationRepository = (*PasskeySessionVerificationRepo)(nil)
