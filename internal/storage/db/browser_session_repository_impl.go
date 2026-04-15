package db

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tiersum/tiersum/internal/storage"
)

// BrowserSessionRepo implements storage.IBrowserSessionRepository.
type BrowserSessionRepo struct {
	db     sqlDB
	driver string
}

func NewBrowserSessionRepo(db sqlDB, driver string) *BrowserSessionRepo {
	return &BrowserSessionRepo{db: db, driver: driver}
}

func (r *BrowserSessionRepo) Create(ctx context.Context, s *storage.BrowserSession) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	q := `INSERT INTO browser_sessions (id, user_id, session_token_hash, fingerprint_hash, ip_prefix, user_agent_norm, timezone, device_alias, expires_at, last_seen_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	args := []interface{}{s.ID, s.UserID, s.SessionTokenHash, s.FingerprintHash, s.IPPrefix, s.UserAgentNorm, s.Timezone, s.DeviceAlias, s.ExpiresAt, s.LastSeenAt, s.CreatedAt}
	if r.driver == "postgres" {
		q = `INSERT INTO browser_sessions (id, user_id, session_token_hash, fingerprint_hash, ip_prefix, user_agent_norm, timezone, device_alias, expires_at, last_seen_at, created_at)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
		args = []interface{}{s.ID, s.UserID, s.SessionTokenHash, s.FingerprintHash, s.IPPrefix, s.UserAgentNorm, s.Timezone, s.DeviceAlias, s.ExpiresAt, s.LastSeenAt, s.CreatedAt}
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *BrowserSessionRepo) scanSession(scanner rowScanner) (*storage.BrowserSession, error) {
	s := &storage.BrowserSession{}
	err := scanner.Scan(&s.ID, &s.UserID, &s.SessionTokenHash, &s.FingerprintHash, &s.IPPrefix, &s.UserAgentNorm, &s.Timezone, &s.DeviceAlias, &s.ExpiresAt, &s.LastSeenAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *BrowserSessionRepo) GetByID(ctx context.Context, sessionID string) (*storage.BrowserSession, error) {
	q := `SELECT id, user_id, session_token_hash, fingerprint_hash, ip_prefix, user_agent_norm, timezone, device_alias, expires_at, last_seen_at, created_at
		FROM browser_sessions WHERE id = ?`
	args := []interface{}{sessionID}
	if r.driver == "postgres" {
		q = `SELECT id::text, user_id::text, session_token_hash, fingerprint_hash, ip_prefix, user_agent_norm, timezone, device_alias, expires_at, last_seen_at, created_at
			FROM browser_sessions WHERE id = $1::uuid`
	}
	row := r.db.QueryRowContext(ctx, q, args...)
	return r.scanSession(row)
}

func (r *BrowserSessionRepo) GetBySessionTokenHash(ctx context.Context, sessionTokenHashHex string) (*storage.BrowserSession, error) {
	q := `SELECT id, user_id, session_token_hash, fingerprint_hash, ip_prefix, user_agent_norm, timezone, device_alias, expires_at, last_seen_at, created_at
		FROM browser_sessions WHERE session_token_hash = ?`
	args := []interface{}{sessionTokenHashHex}
	if r.driver == "postgres" {
		q = `SELECT id::text, user_id::text, session_token_hash, fingerprint_hash, ip_prefix, user_agent_norm, timezone, device_alias, expires_at, last_seen_at, created_at
			FROM browser_sessions WHERE session_token_hash = $1`
	}
	row := r.db.QueryRowContext(ctx, q, args...)
	return r.scanSession(row)
}

func (r *BrowserSessionRepo) UpdateLastSeen(ctx context.Context, sessionID string, at time.Time) error {
	q := `UPDATE browser_sessions SET last_seen_at = ? WHERE id = ?`
	args := []interface{}{at, sessionID}
	if r.driver == "postgres" {
		q = `UPDATE browser_sessions SET last_seen_at = $1 WHERE id = $2::uuid`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *BrowserSessionRepo) UpdateExpiresAt(ctx context.Context, sessionID string, exp time.Time) error {
	q := `UPDATE browser_sessions SET expires_at = ? WHERE id = ?`
	args := []interface{}{exp, sessionID}
	if r.driver == "postgres" {
		q = `UPDATE browser_sessions SET expires_at = $1 WHERE id = $2::uuid`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *BrowserSessionRepo) UpdateDeviceAlias(ctx context.Context, sessionID, alias string) error {
	q := `UPDATE browser_sessions SET device_alias = ? WHERE id = ?`
	args := []interface{}{alias, sessionID}
	if r.driver == "postgres" {
		q = `UPDATE browser_sessions SET device_alias = $1 WHERE id = $2::uuid`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *BrowserSessionRepo) Delete(ctx context.Context, sessionID string) error {
	q := `DELETE FROM browser_sessions WHERE id = ?`
	args := []interface{}{sessionID}
	if r.driver == "postgres" {
		q = `DELETE FROM browser_sessions WHERE id = $1::uuid`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *BrowserSessionRepo) DeleteByUserAndFingerprint(ctx context.Context, userID, fingerprintHashHex string) error {
	q := `DELETE FROM browser_sessions WHERE user_id = ? AND fingerprint_hash = ?`
	args := []interface{}{userID, fingerprintHashHex}
	if r.driver == "postgres" {
		q = `DELETE FROM browser_sessions WHERE user_id = $1::uuid AND fingerprint_hash = $2`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *BrowserSessionRepo) DeleteAllForUser(ctx context.Context, userID string) error {
	q := `DELETE FROM browser_sessions WHERE user_id = ?`
	args := []interface{}{userID}
	if r.driver == "postgres" {
		q = `DELETE FROM browser_sessions WHERE user_id = $1::uuid`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *BrowserSessionRepo) ListByUser(ctx context.Context, userID string) ([]storage.BrowserSession, error) {
	q := `SELECT id, user_id, session_token_hash, fingerprint_hash, ip_prefix, user_agent_norm, timezone, device_alias, expires_at, last_seen_at, created_at
		FROM browser_sessions WHERE user_id = ? ORDER BY last_seen_at DESC`
	args := []interface{}{userID}
	if r.driver == "postgres" {
		q = `SELECT id::text, user_id::text, session_token_hash, fingerprint_hash, ip_prefix, user_agent_norm, timezone, device_alias, expires_at, last_seen_at, created_at
			FROM browser_sessions WHERE user_id = $1::uuid ORDER BY last_seen_at DESC`
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []storage.BrowserSession
	for rows.Next() {
		s, err := r.scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func (r *BrowserSessionRepo) ListAllWithUsername(ctx context.Context) ([]storage.BrowserSessionAdminListRow, error) {
	q := `SELECT bs.id, bs.user_id, bs.session_token_hash, bs.fingerprint_hash, bs.ip_prefix, bs.user_agent_norm, bs.timezone, bs.device_alias, bs.expires_at, bs.last_seen_at, bs.created_at, u.username
		FROM browser_sessions bs INNER JOIN users u ON u.id = bs.user_id ORDER BY bs.last_seen_at DESC`
	if r.driver == "postgres" {
		q = `SELECT bs.id::text, bs.user_id::text, bs.session_token_hash, bs.fingerprint_hash, bs.ip_prefix, bs.user_agent_norm, bs.timezone, bs.device_alias, bs.expires_at, bs.last_seen_at, bs.created_at, u.username
			FROM browser_sessions bs INNER JOIN users u ON u.id = bs.user_id ORDER BY bs.last_seen_at DESC`
	}
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []storage.BrowserSessionAdminListRow
	for rows.Next() {
		var row storage.BrowserSessionAdminListRow
		s := &row.BrowserSession
		if err := rows.Scan(&s.ID, &s.UserID, &s.SessionTokenHash, &s.FingerprintHash, &s.IPPrefix, &s.UserAgentNorm, &s.Timezone, &s.DeviceAlias, &s.ExpiresAt, &s.LastSeenAt, &s.CreatedAt, &row.Username); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *BrowserSessionRepo) CountByUser(ctx context.Context, userID string) (int, error) {
	q := `SELECT COUNT(*) FROM browser_sessions WHERE user_id = ?`
	args := []interface{}{userID}
	if r.driver == "postgres" {
		q = `SELECT COUNT(*) FROM browser_sessions WHERE user_id = $1::uuid`
	}
	var n int
	err := r.db.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, err
}

func (r *BrowserSessionRepo) CountActiveDistinctFingerprints(ctx context.Context, userID string, now time.Time) (int, error) {
	q := `SELECT COUNT(DISTINCT fingerprint_hash) FROM browser_sessions WHERE user_id = ? AND expires_at > ?`
	args := []interface{}{userID, now}
	if r.driver == "postgres" {
		q = `SELECT COUNT(DISTINCT fingerprint_hash) FROM browser_sessions WHERE user_id = $1::uuid AND expires_at > $2`
	}
	var n int
	err := r.db.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, err
}

func (r *BrowserSessionRepo) HasActiveSessionWithFingerprint(ctx context.Context, userID, fingerprintHashHex string, now time.Time) (bool, error) {
	q := `SELECT COUNT(*) FROM browser_sessions WHERE user_id = ? AND fingerprint_hash = ? AND expires_at > ?`
	args := []interface{}{userID, fingerprintHashHex, now}
	if r.driver == "postgres" {
		q = `SELECT COUNT(*) FROM browser_sessions WHERE user_id = $1::uuid AND fingerprint_hash = $2 AND expires_at > $3`
	}
	var n int
	if err := r.db.QueryRowContext(ctx, q, args...).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}
