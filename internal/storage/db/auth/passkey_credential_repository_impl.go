package auth

import (
	"context"
	"fmt"
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
	vals := shared.PlaceholdersCSVWithPGCasts(r.driver, []string{"uuid", "uuid", "", "", "", "", "", ""})
	q := fmt.Sprintf(`INSERT INTO passkey_credentials (id, user_id, credential_id_b64, public_key_b64, sign_count, device_name, last_used_at, created_at)
		VALUES (%s)`, vals)
	args := []any{c.ID, c.UserID, c.CredentialIDB64, c.PublicKeyB64, c.SignCount, c.DeviceName, c.LastUsedAt, c.CreatedAt}
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

func (r *PasskeyCredentialRepo) selectCols() string {
	if shared.DriverIsPostgres(r.driver) {
		return `id::text, user_id::text, credential_id_b64, public_key_b64, sign_count, device_name, last_used_at, created_at`
	}
	return `id, user_id, credential_id_b64, public_key_b64, sign_count, device_name, last_used_at, created_at`
}

func (r *PasskeyCredentialRepo) ListByUser(ctx context.Context, userID string) ([]storage.PasskeyCredential, error) {
	cols := r.selectCols()
	ph := shared.Placeholder(r.driver, 1, "uuid")
	q := fmt.Sprintf(`SELECT %s FROM passkey_credentials WHERE user_id = %s ORDER BY created_at DESC`, cols, ph)
	rows, err := r.db.QueryContext(ctx, q, userID)
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
	cols := r.selectCols()
	ph := shared.Placeholder(r.driver, 1, "uuid")
	q := fmt.Sprintf(`SELECT %s FROM passkey_credentials WHERE id = %s`, cols, ph)
	return r.scan(r.db.QueryRowContext(ctx, q, id))
}

func (r *PasskeyCredentialRepo) GetByCredentialID(ctx context.Context, credentialIDB64 string) (*storage.PasskeyCredential, error) {
	cols := r.selectCols()
	ph := shared.Placeholder(r.driver, 1, "")
	q := fmt.Sprintf(`SELECT %s FROM passkey_credentials WHERE credential_id_b64 = %s`, cols, ph)
	return r.scan(r.db.QueryRowContext(ctx, q, credentialIDB64))
}

func (r *PasskeyCredentialRepo) UpdateSignCountAndLastUsed(ctx context.Context, id string, signCount int64, at time.Time) error {
	ph1 := shared.Placeholder(r.driver, 1, "")
	ph2 := shared.Placeholder(r.driver, 2, "")
	ph3 := shared.Placeholder(r.driver, 3, "uuid")
	q := fmt.Sprintf(`UPDATE passkey_credentials SET sign_count = %s, last_used_at = %s WHERE id = %s`, ph1, ph2, ph3)
	_, err := r.db.ExecContext(ctx, q, signCount, at, id)
	return err
}

func (r *PasskeyCredentialRepo) Delete(ctx context.Context, id string) error {
	ph := shared.Placeholder(r.driver, 1, "uuid")
	q := fmt.Sprintf(`DELETE FROM passkey_credentials WHERE id = %s`, ph)
	_, err := r.db.ExecContext(ctx, q, id)
	return err
}

var _ storage.IPasskeyCredentialRepository = (*PasskeyCredentialRepo)(nil)
