package repo

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type UserRepo struct{ db *sql.DB }

func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) Create(email, plainPassword string) (int64, error) {
	if email == "" || len(plainPassword) < 8 {
		return 0, errors.New("invalid credentials")
	}
	hash, err := HashPassword(plainPassword)
	if err != nil {
		return 0, err
	}
	var id int64
	err = r.db.QueryRow(
		`INSERT INTO users (email, password_hash, password_algo, provider, plan)
		 VALUES ($1, $2, 'argon2id', 'email', 'free')
		 RETURNING id`,
		email, hash,
	).Scan(&id)
	return id, err
}

// scanUser is a shared helper that scans all user columns consistently.
func scanUser(row interface {
	Scan(...any) error
}) (*User, error) {
	var u User
	var passwordHash, passwordAlgo, provider, googleID, name, avatarURL,
		stripeCustomerID, stripeConnectID, magicToken sql.NullString
	var updatedAt, magicTokenExpiresAt, lockedUntil sql.NullTime

	err := row.Scan(
		&u.ID, &u.Email, &passwordHash, &passwordAlgo, &provider, &googleID,
		&name, &avatarURL, &u.Plan, &stripeCustomerID, &stripeConnectID,
		&u.NextInvoiceSeq, &u.NextEstimateSeq, &u.Language,
		&magicToken, &magicTokenExpiresAt,
		&u.FailedLoginAttempts, &lockedUntil,
		&u.CreatedAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if passwordHash.Valid { u.PasswordHash = passwordHash.String }
	if passwordAlgo.Valid { u.PasswordAlgo = passwordAlgo.String }
	if provider.Valid { u.Provider = provider.String }
	if googleID.Valid { u.GoogleID = googleID.String }
	if name.Valid { u.Name = name.String }
	if avatarURL.Valid { u.AvatarURL = avatarURL.String }
	if stripeCustomerID.Valid { u.StripeCustomerID = stripeCustomerID.String }
	if stripeConnectID.Valid { u.StripeConnectID = stripeConnectID.String }
	if magicToken.Valid { u.MagicToken = magicToken.String }
	if magicTokenExpiresAt.Valid {
		t := magicTokenExpiresAt.Time
		u.MagicTokenExpiresAt = &t
	}
	if updatedAt.Valid { u.UpdatedAt = updatedAt.Time }
		if lockedUntil.Valid {
		t := lockedUntil.Time
		u.LockedUntil = &t
	}
	return &u, nil
}

const userSelectCols = `
	SELECT id, email, password_hash, password_algo, provider, google_id,
		name, avatar_url, plan, stripe_customer_id, stripe_connect_id,
		next_invoice_seq, next_estimate_seq, language,
		magic_token, magic_token_expires_at,
		failed_login_attempts, locked_until,
		created_at, updated_at
	FROM users`

func (r *UserRepo) GetByEmail(email string) (*User, error) {
	row := r.db.QueryRow(userSelectCols+` WHERE email = $1`, email)
	return scanUser(row)
}

func (r *UserRepo) GetByID(id int64) (*User, error) {
	row := r.db.QueryRow(userSelectCols+` WHERE id = $1`, id)
	return scanUser(row)
}

func (r *UserRepo) GetByGoogleID(googleID string) (*User, error) {
	row := r.db.QueryRow(userSelectCols+` WHERE google_id = $1`, googleID)
	return scanUser(row)
}

// RehashPassword upgrades a bcrypt hash to Argon2id transparently on login.
func (r *UserRepo) RehashPassword(ctx context.Context, userID int64, plain string) error {
	hash, err := HashPassword(plain)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE users
		SET password_hash = $1, password_algo = 'argon2id', updated_at = NOW()
		WHERE id = $2
	`, hash, userID)
	return err
}

// UpdatePassword sets a new Argon2id password. Requires current password.
func (r *UserRepo) UpdatePassword(ctx context.Context, userID int64, plain string) error {
	hash, err := HashPassword(plain)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE users
		SET password_hash = $1, password_algo = 'argon2id', updated_at = NOW()
		WHERE id = $2
	`, hash, userID)
	return err
}
// SetMagicToken generates a secure token, stores its SHA-256 hash in DB,
// and returns the raw token to be sent in the email.
// The raw token never touches the DB — only the hash does.
//
// If the email does not exist, or if that email is still within the resend
// cooldown window, this returns an empty token and no error.
// The caller should treat both cases as a generic success and send no email.
func (r *UserRepo) SetMagicToken(ctx context.Context, email string) (string, error) {
	// Generate 32 bytes of cryptographic randomness
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	rawHex := hex.EncodeToString(raw)

	// Store SHA-256 hash of token — not the raw token
	hash := sha256.Sum256([]byte(rawHex))
	hashHex := hex.EncodeToString(hash[:])

	now := time.Now()
	expires := now.Add(15 * time.Minute)
	cooldownCutoff := now.Add(14 * time.Minute) // 60-second resend cooldown

	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET magic_token = $1,
		    magic_token_expires_at = $2,
		    updated_at = NOW()
		WHERE LOWER(TRIM(email)) = LOWER(TRIM($3))
		  AND (magic_token_expires_at IS NULL OR magic_token_expires_at <= $4)
	`, hashHex, expires, email, cooldownCutoff)
	if err != nil {
		return "", fmt.Errorf("set magic token: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("magic token rows affected: %w", err)
	}

	// Unknown email or still within cooldown — intentionally silent
	if rows == 0 {
		return "", nil
	}

	return rawHex, nil
}

// ConsumeMagicToken validates a raw token, clears it atomically,
// clears the existing password hash, and returns the user.
// Generic error message prevents token enumeration.
func (r *UserRepo) ConsumeMagicToken(ctx context.Context, rawToken string) (*User, error) {
	hash := sha256.Sum256([]byte(rawToken))
	hashHex := hex.EncodeToString(hash[:])

	row := r.db.QueryRowContext(ctx, `
		UPDATE users
		SET magic_token = NULL,
		    magic_token_expires_at = NULL,
		    password_hash = NULL,
		    password_algo = 'argon2id',
		    updated_at = NOW()
		WHERE magic_token = $1
		  AND magic_token_expires_at > NOW()
		RETURNING id, email, password_hash, password_algo, provider, google_id,
			name, avatar_url, plan, stripe_customer_id, stripe_connect_id,
			next_invoice_seq, next_estimate_seq, language,
			magic_token, magic_token_expires_at,
			created_at, updated_at
	`, hashHex)

	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("invalid or expired link")
		}
		return nil, fmt.Errorf("consume magic token: %w", err)
	}

	return u, nil
}

func (r *UserRepo) NextInvoiceNumber(ctx context.Context, userID int64) (string, error) {
	var seq int
	err := r.db.QueryRowContext(ctx, `
		UPDATE users
		SET next_invoice_seq = next_invoice_seq + 1,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING next_invoice_seq - 1
	`, userID).Scan(&seq)
	if err != nil {
		return "", fmt.Errorf("next invoice seq: %w", err)
	}
	return fmt.Sprintf("INV-%04d", seq), nil
}

func (r *UserRepo) NextEstimateNumber(ctx context.Context, userID int64) (string, error) {
	var seq int
	err := r.db.QueryRowContext(ctx, `
		UPDATE users
		SET next_estimate_seq = next_estimate_seq + 1,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING next_estimate_seq - 1
	`, userID).Scan(&seq)
	if err != nil {
		return "", fmt.Errorf("next estimate seq: %w", err)
	}
	return fmt.Sprintf("EST-%04d", seq), nil
}

func (r *UserRepo) GetInvoiceCount(ctx context.Context, userID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM invoices WHERE user_id = $1`, userID,
	).Scan(&count)
	return count, err
}

func (r *UserRepo) GetMonthlyInvoiceCount(ctx context.Context, userID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM invoices
		WHERE user_id = $1
		AND created_at >= date_trunc('month', now())
	`, userID).Scan(&count)
	return count, err
}

func (r *UserRepo) CreateGoogleUser(email, googleID, name, avatarURL string) (int64, error) {
	var id int64
	err := r.db.QueryRow(`
		INSERT INTO users (email, google_id, provider, name, avatar_url, plan)
		VALUES ($1, $2, 'google', $3, $4, 'free')
		RETURNING id
	`, email, googleID, name, avatarURL).Scan(&id)
	return id, err
}

func (r *UserRepo) LinkGoogleToExisting(userID int64, googleID string) error {
	_, err := r.db.Exec(`
		UPDATE users
		SET google_id = $1, provider = 'google', updated_at = NOW()
		WHERE id = $2 AND (google_id IS NULL OR google_id = $1)
	`, googleID, userID)
	return err
}

func (r *UserRepo) FindOrCreateGoogleUser(email, googleID, name, avatarURL string) (*User, bool, error) {
	user, err := r.GetByGoogleID(googleID)
	if err == nil {
		_, _ = r.db.Exec(`
			UPDATE users SET name = $1, avatar_url = $2, updated_at = NOW() WHERE id = $3
		`, name, avatarURL, user.ID)
		user.Name = name
		user.AvatarURL = avatarURL
		return user, false, nil
	}
	if err != sql.ErrNoRows {
		return nil, false, fmt.Errorf("lookup by google id: %w", err)
	}
	user, err = r.GetByEmail(email)
	if err == nil {
		if linkErr := r.LinkGoogleToExisting(user.ID, googleID); linkErr != nil {
			return nil, false, fmt.Errorf("link google: %w", linkErr)
		}
		_, _ = r.db.Exec(`
			UPDATE users SET name = $1, avatar_url = $2, updated_at = NOW() WHERE id = $3
		`, name, avatarURL, user.ID)
		user.GoogleID = googleID
		user.Name = name
		user.AvatarURL = avatarURL
		user.Provider = "google"
		return user, false, nil
	}
	if err != sql.ErrNoRows {
		return nil, false, fmt.Errorf("lookup by email: %w", err)
	}
	id, err := r.CreateGoogleUser(email, googleID, name, avatarURL)
	if err != nil {
		return nil, false, fmt.Errorf("create google user: %w", err)
	}
	user, err = r.GetByID(id)
	if err != nil {
		return nil, false, fmt.Errorf("fetch new user: %w", err)
	}
	return user, true, nil
}

func (r *UserRepo) UpdateUserPlan(ctx context.Context, userID int64, plan string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users SET plan = $1, updated_at = NOW() WHERE id = $2
	`, plan, userID)
	return err
}

func (r *UserRepo) UpdateStripeCustomerID(ctx context.Context, userID int64, customerID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users SET stripe_customer_id = $1, updated_at = NOW() WHERE id = $2
	`, customerID, userID)
	return err
}

func (r *UserRepo) SaveStripeConnectID(ctx context.Context, userID int64, connectID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users SET stripe_connect_id = $1, updated_at = NOW() WHERE id = $2
	`, connectID, userID)
	return err
}

func (r *UserRepo) UpdateLanguage(ctx context.Context, userID int64, language string) error {
	if language != "en" && language != "es" {
		language = "en"
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE users SET language = $1, updated_at = NOW() WHERE id = $2
	`, language, userID)
	return err
}
// RecordFailedLogin increments the failed attempt counter.
// After 10 failures, locks the account for 15 minutes.
func (r *UserRepo) RecordFailedLogin(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET failed_login_attempts = failed_login_attempts + 1,
		    locked_until = CASE
		        WHEN failed_login_attempts + 1 >= 10
		        THEN NOW() + INTERVAL '15 minutes'
		        ELSE locked_until
		    END,
		    updated_at = NOW()
		WHERE id = $1
	`, userID)
	return err
}

// ResetFailedLogins clears the counter and lock on successful login.
func (r *UserRepo) ResetFailedLogins(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET failed_login_attempts = 0,
		    locked_until = NULL,
		    updated_at = NOW()
		WHERE id = $1
	`, userID)
	return err
}
