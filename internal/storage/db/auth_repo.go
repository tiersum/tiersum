package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tiersum/tiersum/internal/storage"
)

// SystemAuthStateRepo implements storage.ISystemAuthStateRepository.
type SystemAuthStateRepo struct {
	db     sqlDB
	driver string
}

func NewSystemAuthStateRepo(db sqlDB, driver string) *SystemAuthStateRepo {
	return &SystemAuthStateRepo{db: db, driver: driver}
}

func (r *SystemAuthStateRepo) Get(ctx context.Context) (*storage.SystemAuthState, error) {
	q := `SELECT initialized_at FROM system_state WHERE id = 1`
	if r.driver == "postgres" {
		q = `SELECT initialized_at FROM system_state WHERE id = 1`
	}
	var t sql.NullTime
	err := r.db.QueryRowContext(ctx, q).Scan(&t)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Missing singleton row (partial migration or pre-v9 DB): treat as uninitialized, not a hard failure.
			if seedErr := r.ensureSingletonRow(ctx); seedErr != nil {
				return nil, seedErr
			}
			return &storage.SystemAuthState{}, nil
		}
		return nil, err
	}
	st := &storage.SystemAuthState{}
	if t.Valid {
		st.InitializedAt = &t.Time
	}
	return st, nil
}

// ensureSingletonRow inserts the default system_state row if absent (id=1).
func (r *SystemAuthStateRepo) ensureSingletonRow(ctx context.Context) error {
	var q string
	if r.driver == "postgres" {
		q = `INSERT INTO system_state (id, initialized_at) VALUES (1, NULL) ON CONFLICT (id) DO NOTHING`
	} else {
		q = `INSERT OR IGNORE INTO system_state (id, initialized_at) VALUES (1, NULL)`
	}
	_, err := r.db.ExecContext(ctx, q)
	return err
}

func (r *SystemAuthStateRepo) MarkInitialized(ctx context.Context) error {
	if err := r.ensureSingletonRow(ctx); err != nil {
		return err
	}
	now := time.Now().UTC()
	q := `UPDATE system_state SET initialized_at = ? WHERE id = 1 AND initialized_at IS NULL`
	args := []interface{}{now}
	if r.driver == "postgres" {
		q = `UPDATE system_state SET initialized_at = $1 WHERE id = 1 AND initialized_at IS NULL`
	}
	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("system already initialized or state row missing")
	}
	return nil
}

// AuthUserRepo implements storage.IAuthUserRepository.
type AuthUserRepo struct {
	db     sqlDB
	driver string
}

func NewAuthUserRepo(db sqlDB, driver string) *AuthUserRepo {
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

func (r *AuthUserRepo) scanUser(scanner rowScanner) (*storage.AuthUser, error) {
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

type rowScanner interface {
	Scan(dest ...interface{}) error
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

// APIKeyAuditRepo implements storage.IAPIKeyAuditRepository.
type APIKeyAuditRepo struct {
	db     sqlDB
	driver string
}

func NewAPIKeyAuditRepo(db sqlDB, driver string) *APIKeyAuditRepo {
	return &APIKeyAuditRepo{db: db, driver: driver}
}

func (r *APIKeyAuditRepo) Insert(ctx context.Context, apiKeyID, method, path, clientIP string, at time.Time) error {
	q := `INSERT INTO api_key_audit (api_key_id, method, path, client_ip, called_at) VALUES (?, ?, ?, ?, ?)`
	args := []interface{}{apiKeyID, method, path, clientIP, at}
	if r.driver == "postgres" {
		q = `INSERT INTO api_key_audit (api_key_id, method, path, client_ip, called_at) VALUES ($1::uuid, $2, $3, $4, $5)`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *APIKeyAuditRepo) CountSince(ctx context.Context, apiKeyID string, since time.Time) (int64, error) {
	q := `SELECT COUNT(*) FROM api_key_audit WHERE api_key_id = ? AND called_at >= ?`
	args := []interface{}{apiKeyID, since}
	if r.driver == "postgres" {
		q = `SELECT COUNT(*) FROM api_key_audit WHERE api_key_id = $1::uuid AND called_at >= $2`
	}
	var n int64
	err := r.db.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, err
}

func (r *APIKeyAuditRepo) CountsPerKeySince(ctx context.Context, since time.Time) (map[string]int64, error) {
	q := `SELECT api_key_id, COUNT(*) FROM api_key_audit WHERE called_at >= ? GROUP BY api_key_id`
	args := []interface{}{since}
	if r.driver == "postgres" {
		q = `SELECT api_key_id::text, COUNT(*) FROM api_key_audit WHERE called_at >= $1 GROUP BY api_key_id`
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int64)
	for rows.Next() {
		var kid string
		var c int64
		if err := rows.Scan(&kid, &c); err != nil {
			return nil, err
		}
		out[kid] = c
	}
	return out, rows.Err()
}
