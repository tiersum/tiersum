package auth

import (
	"context"
	"database/sql"
	"fmt"
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
	ctx, span := shared.WithRepoSpan(ctx, "AuthUserRepo.Create")
	if span != nil { defer span.End() }
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	shared.SetSpanInputID(span, u.ID)
	now := time.Now().UTC()
	u.CreatedAt = now
	u.UpdatedAt = now
	vals := shared.PlaceholdersCSVWithPGCasts(r.driver, []string{"uuid", "", "", "", "", "", "", "", ""})
	q := fmt.Sprintf(`INSERT INTO users (id, username, role, access_token_hash, token_expiry_mode, max_devices, token_valid_until, created_at, updated_at)
		VALUES (%s)`, vals)
	args := []interface{}{u.ID, u.Username, u.Role, u.AccessTokenHash, u.TokenExpiryMode, u.MaxDevices, u.TokenValidUntil, u.CreatedAt, u.UpdatedAt}
	_, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		shared.SetSpanStatus(span, err)
		return err
	}
	shared.SetSpanStatus(span, nil)
	return nil
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

func (r *AuthUserRepo) selectUserCols() string {
	if shared.DriverIsPostgres(r.driver) {
		return `id::text, username, role, access_token_hash, token_expiry_mode, max_devices, token_valid_until, created_at, updated_at`
	}
	return `id, username, role, access_token_hash, token_expiry_mode, max_devices, token_valid_until, created_at, updated_at`
}

func (r *AuthUserRepo) GetByID(ctx context.Context, id string) (*storage.AuthUser, error) {
	ctx, span := shared.WithRepoSpan(ctx, "AuthUserRepo.GetByID")
	if span != nil { defer span.End() }
	shared.SetSpanInputID(span, id)
	cols := r.selectUserCols()
	ph := shared.Placeholder(r.driver, 1, "uuid")
	q := fmt.Sprintf(`SELECT %s FROM users WHERE id = %s`, cols, ph)
	u, err := r.scanUser(r.db.QueryRowContext(ctx, q, id))
	if err != nil {
		shared.SetSpanStatus(span, err)
		return nil, err
	}
	shared.SetSpanOutputString(span, "username", u.Username)
	shared.SetSpanStatus(span, nil)
	return u, nil
}

func (r *AuthUserRepo) GetByUsername(ctx context.Context, username string) (*storage.AuthUser, error) {
	ctx, span := shared.WithRepoSpan(ctx, "AuthUserRepo.GetByUsername")
	if span != nil { defer span.End() }
	shared.SetSpanInputString(span, "username", username)
	cols := r.selectUserCols()
	ph := shared.Placeholder(r.driver, 1, "")
	q := fmt.Sprintf(`SELECT %s FROM users WHERE username = %s`, cols, ph)
	u, err := r.scanUser(r.db.QueryRowContext(ctx, q, username))
	if err != nil {
		shared.SetSpanStatus(span, err)
		return nil, err
	}
	shared.SetSpanOutputString(span, "username", u.Username)
	shared.SetSpanStatus(span, nil)
	return u, nil
}

func (r *AuthUserRepo) GetByAccessTokenHash(ctx context.Context, hashHex string) (*storage.AuthUser, error) {
	ctx, span := shared.WithRepoSpan(ctx, "AuthUserRepo.GetByAccessTokenHash")
	if span != nil { defer span.End() }
	shared.SetSpanInputString(span, "hash", hashHex)
	cols := r.selectUserCols()
	ph := shared.Placeholder(r.driver, 1, "")
	q := fmt.Sprintf(`SELECT %s FROM users WHERE access_token_hash = %s`, cols, ph)
	u, err := r.scanUser(r.db.QueryRowContext(ctx, q, hashHex))
	if err != nil {
		shared.SetSpanStatus(span, err)
		return nil, err
	}
	shared.SetSpanOutputString(span, "username", u.Username)
	shared.SetSpanStatus(span, nil)
	return u, nil
}

func (r *AuthUserRepo) UpdateAccessToken(ctx context.Context, userID, accessTokenHashHex string, validUntil *time.Time) error {
	ctx, span := shared.WithRepoSpan(ctx, "AuthUserRepo.UpdateAccessToken")
	if span != nil { defer span.End() }
	shared.SetSpanInputID(span, userID)
	now := time.Now().UTC()
	ph1 := shared.Placeholder(r.driver, 1, "")
	ph2 := shared.Placeholder(r.driver, 2, "")
	ph3 := shared.Placeholder(r.driver, 3, "")
	ph4 := shared.Placeholder(r.driver, 4, "uuid")
	q := fmt.Sprintf(`UPDATE users SET access_token_hash = %s, token_valid_until = %s, updated_at = %s WHERE id = %s`, ph1, ph2, ph3, ph4)
	_, err := r.db.ExecContext(ctx, q, accessTokenHashHex, validUntil, now, userID)
	if err != nil {
		shared.SetSpanStatus(span, err)
		return err
	}
	shared.SetSpanStatus(span, nil)
	return nil
}

func (r *AuthUserRepo) UpdateTokenExpiryMode(ctx context.Context, userID, mode string) error {
	ctx, span := shared.WithRepoSpan(ctx, "AuthUserRepo.UpdateTokenExpiryMode")
	if span != nil { defer span.End() }
	shared.SetSpanInputID(span, userID)
	shared.SetSpanInputString(span, "mode", mode)
	now := time.Now().UTC()
	ph1 := shared.Placeholder(r.driver, 1, "")
	ph2 := shared.Placeholder(r.driver, 2, "")
	ph3 := shared.Placeholder(r.driver, 3, "uuid")
	q := fmt.Sprintf(`UPDATE users SET token_expiry_mode = %s, updated_at = %s WHERE id = %s`, ph1, ph2, ph3)
	_, err := r.db.ExecContext(ctx, q, mode, now, userID)
	if err != nil {
		shared.SetSpanStatus(span, err)
		return err
	}
	shared.SetSpanStatus(span, nil)
	return nil
}

func (r *AuthUserRepo) UpdateTokenValidUntil(ctx context.Context, userID string, validUntil *time.Time) error {
	ctx, span := shared.WithRepoSpan(ctx, "AuthUserRepo.UpdateTokenValidUntil")
	if span != nil { defer span.End() }
	shared.SetSpanInputID(span, userID)
	now := time.Now().UTC()
	ph1 := shared.Placeholder(r.driver, 1, "")
	ph2 := shared.Placeholder(r.driver, 2, "")
	ph3 := shared.Placeholder(r.driver, 3, "uuid")
	q := fmt.Sprintf(`UPDATE users SET token_valid_until = %s, updated_at = %s WHERE id = %s`, ph1, ph2, ph3)
	_, err := r.db.ExecContext(ctx, q, validUntil, now, userID)
	if err != nil {
		shared.SetSpanStatus(span, err)
		return err
	}
	shared.SetSpanStatus(span, nil)
	return nil
}

func (r *AuthUserRepo) List(ctx context.Context) ([]storage.AuthUser, error) {
	ctx, span := shared.WithRepoSpan(ctx, "AuthUserRepo.List")
	if span != nil { defer span.End() }
	q := fmt.Sprintf(`SELECT %s FROM users ORDER BY username`, r.selectUserCols())
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		shared.SetSpanStatus(span, err)
		return nil, err
	}
	defer rows.Close()
	var out []storage.AuthUser
	for rows.Next() {
		u, err := r.scanUser(rows)
		if err != nil {
			shared.SetSpanStatus(span, err)
			return nil, err
		}
		out = append(out, *u)
	}
	if err := rows.Err(); err != nil {
		shared.SetSpanStatus(span, err)
		return nil, err
	}
	shared.SetSpanOutputCount(span, len(out))
	shared.SetSpanOutputStrings(span, "usernames", shared.CollectIDs(out, func(u storage.AuthUser) string { return u.Username }))
	shared.SetSpanStatus(span, nil)
	return out, nil
}
