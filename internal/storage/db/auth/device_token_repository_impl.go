package auth

import (
	"context"
	"fmt"
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
	ctx, span := shared.WithRepoSpan(ctx, "DeviceTokenRepo.Create")
	if span != nil { defer span.End() }

	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	shared.SetSpanInputID(span, t.ID)
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	vals := shared.PlaceholdersCSVWithPGCasts(r.driver, []string{"uuid", "uuid", "", "", "", "", "", "", "", ""})
	q := fmt.Sprintf(`INSERT INTO device_tokens (id, user_id, token_hash, device_name, ip_prefix, user_agent_norm, last_used_at, expires_at, revoked_at, created_at)
		VALUES (%s)`, vals)
	args := []any{t.ID, t.UserID, t.TokenHash, t.DeviceName, t.IPPrefix, t.UserAgentNorm, t.LastUsedAt, t.ExpiresAt, t.RevokedAt, t.CreatedAt}
	_, err := r.db.ExecContext(ctx, q, args...)
	shared.SetSpanStatus(span, err)
	return err
}

func (r *DeviceTokenRepo) scan(scanner shared.RowScanner) (*storage.DeviceToken, error) {
	var t storage.DeviceToken
	if err := scanner.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.DeviceName, &t.IPPrefix, &t.UserAgentNorm, &t.LastUsedAt, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *DeviceTokenRepo) selectCols() string {
	if shared.DriverIsPostgres(r.driver) {
		return `id::text, user_id::text, token_hash, device_name, ip_prefix, user_agent_norm, last_used_at, expires_at, revoked_at, created_at`
	}
	return `id, user_id, token_hash, device_name, ip_prefix, user_agent_norm, last_used_at, expires_at, revoked_at, created_at`
}

func (r *DeviceTokenRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*storage.DeviceToken, error) {
	ctx, span := shared.WithRepoSpan(ctx, "DeviceTokenRepo.GetByTokenHash")
	if span != nil { defer span.End() }
	shared.SetSpanInputString(span, "token_hash", tokenHash)

	cols := r.selectCols()
	ph := shared.Placeholder(r.driver, 1, "")
	q := fmt.Sprintf(`SELECT %s FROM device_tokens WHERE token_hash = %s`, cols, ph)
	t, err := r.scan(r.db.QueryRowContext(ctx, q, tokenHash))
	if err != nil {
		shared.SetSpanStatus(span, err)
		return nil, err
	}
	shared.SetSpanOutputID(span, t.ID)
	shared.SetSpanStatus(span, nil)
	return t, nil
}

func (r *DeviceTokenRepo) ListByUser(ctx context.Context, userID string) ([]storage.DeviceToken, error) {
	ctx, span := shared.WithRepoSpan(ctx, "DeviceTokenRepo.ListByUser")
	if span != nil { defer span.End() }
	shared.SetSpanInputID(span, userID)

	cols := r.selectCols()
	ph := shared.Placeholder(r.driver, 1, "uuid")
	q := fmt.Sprintf(`SELECT %s FROM device_tokens WHERE user_id = %s ORDER BY created_at DESC`, cols, ph)
	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		shared.SetSpanStatus(span, err)
		return nil, err
	}
	defer rows.Close()
	out := make([]storage.DeviceToken, 0)
	for rows.Next() {
		t, err := r.scan(rows)
		if err != nil {
			shared.SetSpanStatus(span, err)
			return nil, err
		}
		out = append(out, *t)
	}
	if err := rows.Err(); err != nil {
		shared.SetSpanStatus(span, err)
		return nil, err
	}
	shared.SetSpanOutputCount(span, len(out))
	ids := make([]string, 0, len(out))
	for _, t := range out {
		ids = append(ids, t.ID)
	}
	shared.SetSpanOutputIDs(span, ids)
	shared.SetSpanStatus(span, nil)
	return out, nil
}

func (r *DeviceTokenRepo) TouchUse(ctx context.Context, id string, at time.Time) error {
	ctx, span := shared.WithRepoSpan(ctx, "DeviceTokenRepo.TouchUse")
	if span != nil { defer span.End() }
	shared.SetSpanInputID(span, id)

	ph1 := shared.Placeholder(r.driver, 1, "")
	ph2 := shared.Placeholder(r.driver, 2, "uuid")
	q := fmt.Sprintf(`UPDATE device_tokens SET last_used_at = %s WHERE id = %s`, ph1, ph2)
	_, err := r.db.ExecContext(ctx, q, at, id)
	shared.SetSpanStatus(span, err)
	return err
}

func (r *DeviceTokenRepo) Revoke(ctx context.Context, id string, at time.Time) error {
	ctx, span := shared.WithRepoSpan(ctx, "DeviceTokenRepo.Revoke")
	if span != nil { defer span.End() }
	shared.SetSpanInputID(span, id)

	ph1 := shared.Placeholder(r.driver, 1, "")
	ph2 := shared.Placeholder(r.driver, 2, "uuid")
	q := fmt.Sprintf(`UPDATE device_tokens SET revoked_at = %s WHERE id = %s`, ph1, ph2)
	_, err := r.db.ExecContext(ctx, q, at, id)
	shared.SetSpanStatus(span, err)
	return err
}

func (r *DeviceTokenRepo) RevokeAllForUser(ctx context.Context, userID string, at time.Time) error {
	ctx, span := shared.WithRepoSpan(ctx, "DeviceTokenRepo.RevokeAllForUser")
	if span != nil { defer span.End() }
	shared.SetSpanInputID(span, userID)

	ph1 := shared.Placeholder(r.driver, 1, "")
	ph2 := shared.Placeholder(r.driver, 2, "uuid")
	q := fmt.Sprintf(`UPDATE device_tokens SET revoked_at = %s WHERE user_id = %s`, ph1, ph2)
	_, err := r.db.ExecContext(ctx, q, at, userID)
	shared.SetSpanStatus(span, err)
	return err
}

var _ storage.IDeviceTokenRepository = (*DeviceTokenRepo)(nil)
