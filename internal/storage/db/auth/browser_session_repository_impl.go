package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/db/shared"
)

// BrowserSessionRepo implements storage.IBrowserSessionRepository.
type BrowserSessionRepo struct {
	db     shared.SQLDB
	driver string
}

func NewBrowserSessionRepo(db shared.SQLDB, driver string) *BrowserSessionRepo {
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
	vals := shared.PlaceholdersCSVWithPGCasts(r.driver, []string{
		"uuid", "uuid", "", "", "", "", "", "", "", "", "",
	})
	q := fmt.Sprintf(`INSERT INTO browser_sessions (id, user_id, session_token_hash, fingerprint_hash, ip_prefix, user_agent_norm, timezone, device_alias, expires_at, last_seen_at, created_at)
		VALUES (%s)`, vals)
	args := []interface{}{s.ID, s.UserID, s.SessionTokenHash, s.FingerprintHash, s.IPPrefix, s.UserAgentNorm, s.Timezone, s.DeviceAlias, s.ExpiresAt, s.LastSeenAt, s.CreatedAt}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *BrowserSessionRepo) scanSession(scanner shared.RowScanner) (*storage.BrowserSession, error) {
	s := &storage.BrowserSession{}
	err := scanner.Scan(&s.ID, &s.UserID, &s.SessionTokenHash, &s.FingerprintHash, &s.IPPrefix, &s.UserAgentNorm, &s.Timezone, &s.DeviceAlias, &s.ExpiresAt, &s.LastSeenAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *BrowserSessionRepo) sessionSelectCols() string {
	if shared.DriverIsPostgres(r.driver) {
		return `id::text, user_id::text, session_token_hash, fingerprint_hash, ip_prefix, user_agent_norm, timezone, device_alias, expires_at, last_seen_at, created_at`
	}
	return `id, user_id, session_token_hash, fingerprint_hash, ip_prefix, user_agent_norm, timezone, device_alias, expires_at, last_seen_at, created_at`
}

func (r *BrowserSessionRepo) GetByID(ctx context.Context, sessionID string) (*storage.BrowserSession, error) {
	cols := r.sessionSelectCols()
	ph := shared.Placeholder(r.driver, 1, "uuid")
	q := fmt.Sprintf(`SELECT %s FROM browser_sessions WHERE id = %s`, cols, ph)
	row := r.db.QueryRowContext(ctx, q, sessionID)
	return r.scanSession(row)
}

func (r *BrowserSessionRepo) GetBySessionTokenHash(ctx context.Context, sessionTokenHashHex string) (*storage.BrowserSession, error) {
	cols := r.sessionSelectCols()
	ph := shared.Placeholder(r.driver, 1, "")
	q := fmt.Sprintf(`SELECT %s FROM browser_sessions WHERE session_token_hash = %s`, cols, ph)
	row := r.db.QueryRowContext(ctx, q, sessionTokenHashHex)
	return r.scanSession(row)
}

func (r *BrowserSessionRepo) UpdateLastSeen(ctx context.Context, sessionID string, at time.Time) error {
	ph1 := shared.Placeholder(r.driver, 1, "")
	ph2 := shared.Placeholder(r.driver, 2, "uuid")
	q := fmt.Sprintf(`UPDATE browser_sessions SET last_seen_at = %s WHERE id = %s`, ph1, ph2)
	_, err := r.db.ExecContext(ctx, q, at, sessionID)
	return err
}

func (r *BrowserSessionRepo) UpdateExpiresAt(ctx context.Context, sessionID string, exp time.Time) error {
	ph1 := shared.Placeholder(r.driver, 1, "")
	ph2 := shared.Placeholder(r.driver, 2, "uuid")
	q := fmt.Sprintf(`UPDATE browser_sessions SET expires_at = %s WHERE id = %s`, ph1, ph2)
	_, err := r.db.ExecContext(ctx, q, exp, sessionID)
	return err
}

func (r *BrowserSessionRepo) UpdateDeviceAlias(ctx context.Context, sessionID, alias string) error {
	ph1 := shared.Placeholder(r.driver, 1, "")
	ph2 := shared.Placeholder(r.driver, 2, "uuid")
	q := fmt.Sprintf(`UPDATE browser_sessions SET device_alias = %s WHERE id = %s`, ph1, ph2)
	_, err := r.db.ExecContext(ctx, q, alias, sessionID)
	return err
}

func (r *BrowserSessionRepo) Delete(ctx context.Context, sessionID string) error {
	ph := shared.Placeholder(r.driver, 1, "uuid")
	q := fmt.Sprintf(`DELETE FROM browser_sessions WHERE id = %s`, ph)
	_, err := r.db.ExecContext(ctx, q, sessionID)
	return err
}

func (r *BrowserSessionRepo) DeleteByUserAndFingerprint(ctx context.Context, userID, fingerprintHashHex string) error {
	ph1 := shared.Placeholder(r.driver, 1, "uuid")
	ph2 := shared.Placeholder(r.driver, 2, "")
	q := fmt.Sprintf(`DELETE FROM browser_sessions WHERE user_id = %s AND fingerprint_hash = %s`, ph1, ph2)
	_, err := r.db.ExecContext(ctx, q, userID, fingerprintHashHex)
	return err
}

func (r *BrowserSessionRepo) DeleteAllForUser(ctx context.Context, userID string) error {
	ph := shared.Placeholder(r.driver, 1, "uuid")
	q := fmt.Sprintf(`DELETE FROM browser_sessions WHERE user_id = %s`, ph)
	_, err := r.db.ExecContext(ctx, q, userID)
	return err
}

func (r *BrowserSessionRepo) ListByUser(ctx context.Context, userID string) ([]storage.BrowserSession, error) {
	cols := r.sessionSelectCols()
	ph := shared.Placeholder(r.driver, 1, "uuid")
	q := fmt.Sprintf(`SELECT %s FROM browser_sessions WHERE user_id = %s ORDER BY last_seen_at DESC`, cols, ph)
	rows, err := r.db.QueryContext(ctx, q, userID)
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
	idSel, uidSel := "bs.id", "bs.user_id"
	if shared.DriverIsPostgres(r.driver) {
		idSel, uidSel = "bs.id::text", "bs.user_id::text"
	}
	q := fmt.Sprintf(`SELECT %s, %s, bs.session_token_hash, bs.fingerprint_hash, bs.ip_prefix, bs.user_agent_norm, bs.timezone, bs.device_alias, bs.expires_at, bs.last_seen_at, bs.created_at, u.username
		FROM browser_sessions bs INNER JOIN users u ON u.id = bs.user_id ORDER BY bs.last_seen_at DESC`, idSel, uidSel)
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
	ph := shared.Placeholder(r.driver, 1, "uuid")
	q := fmt.Sprintf(`SELECT COUNT(*) FROM browser_sessions WHERE user_id = %s`, ph)
	var n int
	err := r.db.QueryRowContext(ctx, q, userID).Scan(&n)
	return n, err
}

func (r *BrowserSessionRepo) CountActiveDistinctFingerprints(ctx context.Context, userID string, now time.Time) (int, error) {
	ph1 := shared.Placeholder(r.driver, 1, "uuid")
	ph2 := shared.Placeholder(r.driver, 2, "")
	q := fmt.Sprintf(`SELECT COUNT(DISTINCT fingerprint_hash) FROM browser_sessions WHERE user_id = %s AND expires_at > %s`, ph1, ph2)
	var n int
	err := r.db.QueryRowContext(ctx, q, userID, now).Scan(&n)
	return n, err
}

func (r *BrowserSessionRepo) HasActiveSessionWithFingerprint(ctx context.Context, userID, fingerprintHashHex string, now time.Time) (bool, error) {
	ph1 := shared.Placeholder(r.driver, 1, "uuid")
	ph2 := shared.Placeholder(r.driver, 2, "")
	ph3 := shared.Placeholder(r.driver, 3, "")
	q := fmt.Sprintf(`SELECT COUNT(*) FROM browser_sessions WHERE user_id = %s AND fingerprint_hash = %s AND expires_at > %s`, ph1, ph2, ph3)
	var n int
	if err := r.db.QueryRowContext(ctx, q, userID, fingerprintHashHex, now).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

var (
	_ storage.IBrowserSessionCoreRepository        = (*BrowserSessionRepo)(nil)
	_ storage.IBrowserSessionLoginPolicyRepository = (*BrowserSessionRepo)(nil)
	_ storage.IBrowserSessionAdminRepository       = (*BrowserSessionRepo)(nil)
	_ storage.IBrowserSessionRepository            = (*BrowserSessionRepo)(nil)
)
