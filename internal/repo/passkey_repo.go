// internal/repo/passkey_repo.go
package repo

import (
	"context"
	"database/sql"
	"time"
)

// WebAuthnCredential represents a stored passkey credential.
type WebAuthnCredential struct {
	ID           int64
	UserID       int64
	CredentialID []byte
	PublicKey    []byte
	SignCount    uint32
	DeviceName   string
	CreatedAt    time.Time
	LastUsedAt   *time.Time
}

type PasskeyRepo struct{ db *sql.DB }

func NewPasskeyRepo(db *sql.DB) *PasskeyRepo { return &PasskeyRepo{db: db} }

// Create stores a new WebAuthn credential for a user.
func (r *PasskeyRepo) Create(ctx context.Context, cred *WebAuthnCredential) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO webauthn_credentials
			(user_id, credential_id, public_key, sign_count, device_name)
		VALUES ($1, $2, $3, $4, $5)`,
		cred.UserID, cred.CredentialID, cred.PublicKey, cred.SignCount, cred.DeviceName,
	)
	return err
}

// GetByUserID returns all credentials for a given user.
func (r *PasskeyRepo) GetByUserID(ctx context.Context, userID int64) ([]WebAuthnCredential, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, credential_id, public_key, sign_count,
		       device_name, created_at, last_used_at
		FROM webauthn_credentials
		WHERE user_id = $1
		ORDER BY created_at`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var creds []WebAuthnCredential
	for rows.Next() {
		var c WebAuthnCredential
		if err := rows.Scan(
			&c.ID, &c.UserID, &c.CredentialID, &c.PublicKey,
			&c.SignCount, &c.DeviceName, &c.CreatedAt, &c.LastUsedAt,
		); err != nil {
			return nil, err
		}
		creds = append(creds, c)
	}
	return creds, rows.Err()
}

// GetByCredentialID finds a credential by its WebAuthn credential ID.
func (r *PasskeyRepo) GetByCredentialID(ctx context.Context, credID []byte) (*WebAuthnCredential, error) {
	var c WebAuthnCredential
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, credential_id, public_key, sign_count,
		       device_name, created_at, last_used_at
		FROM webauthn_credentials
		WHERE credential_id = $1`, credID,
	).Scan(
		&c.ID, &c.UserID, &c.CredentialID, &c.PublicKey,
		&c.SignCount, &c.DeviceName, &c.CreatedAt, &c.LastUsedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpdateSignCount updates the sign count and last used timestamp after authentication.
func (r *PasskeyRepo) UpdateSignCount(ctx context.Context, credID []byte, newCount uint32) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE webauthn_credentials
		SET sign_count = $1, last_used_at = NOW()
		WHERE credential_id = $2`,
		newCount, credID,
	)
	return err
}

// Delete removes a credential by ID and user ID.
func (r *PasskeyRepo) Delete(ctx context.Context, id int64, userID int64) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM webauthn_credentials
		WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	return err
}

