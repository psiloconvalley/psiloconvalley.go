package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type UserRepo struct{ db *sql.DB }

func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) Create(email, plainPassword string) (int64, error) {
	if email == "" || len(plainPassword) < 8 {
		return 0, errors.New("invalid credentials")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), 12)
	if err != nil {
		return 0, err
	}
	var id int64
	err = r.db.QueryRow(
		`INSERT INTO users (email, password_hash, provider, plan)
		 VALUES ($1, $2, 'email', 'free')
		 RETURNING id`,
		email, string(hash),
	).Scan(&id)
	return id, err
}

func (r *UserRepo) GetByEmail(email string) (*User, error) {
	var u User
	var passwordHash, provider, googleID, name, avatarURL, stripeCustomerID, stripeConnectID sql.NullString
	var updatedAt sql.NullTime
	err := r.db.QueryRow(`
		SELECT id, email, password_hash, provider, google_id,
		name, avatar_url, plan, stripe_customer_id, stripe_connect_id, next_invoice_seq, next_estimate_seq, language, created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(
		&u.ID, &u.Email, &passwordHash, &provider, &googleID,
		&name, &avatarURL, &u.Plan, &stripeCustomerID, &stripeConnectID, &u.NextInvoiceSeq, &u.NextEstimateSeq, &u.Language, &u.CreatedAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if passwordHash.Valid { u.PasswordHash = passwordHash.String }
	if provider.Valid { u.Provider = provider.String }
	if googleID.Valid { u.GoogleID = googleID.String }
	if name.Valid { u.Name = name.String }
	if avatarURL.Valid { u.AvatarURL = avatarURL.String }
	if stripeCustomerID.Valid { u.StripeCustomerID = stripeCustomerID.String }
	if stripeConnectID.Valid { u.StripeConnectID = stripeConnectID.String }
	if updatedAt.Valid { u.UpdatedAt = updatedAt.Time }
	return &u, nil
}

func (r *UserRepo) GetByID(id int64) (*User, error) {
	var u User
	var passwordHash, provider, googleID, name, avatarURL, stripeCustomerID, stripeConnectID sql.NullString
	var updatedAt sql.NullTime
	err := r.db.QueryRow(`
		SELECT id, email, password_hash, provider, google_id,
			name, avatar_url, plan, stripe_customer_id, stripe_connect_id, next_invoice_seq, next_estimate_seq, language, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(
		&u.ID, &u.Email, &passwordHash, &provider, &googleID,
		&name, &avatarURL, &u.Plan, &stripeCustomerID, &stripeConnectID, &u.NextInvoiceSeq, &u.NextEstimateSeq, &u.Language, &u.CreatedAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if passwordHash.Valid { u.PasswordHash = passwordHash.String }
	if provider.Valid { u.Provider = provider.String }
	if googleID.Valid { u.GoogleID = googleID.String }
	if name.Valid { u.Name = name.String }
	if avatarURL.Valid { u.AvatarURL = avatarURL.String }
	if stripeCustomerID.Valid { u.StripeCustomerID = stripeCustomerID.String }
	if stripeConnectID.Valid { u.StripeConnectID = stripeConnectID.String }
	if updatedAt.Valid { u.UpdatedAt = updatedAt.Time }
	return &u, nil
}

func (r *UserRepo) GetByGoogleID(googleID string) (*User, error) {
	var u User
	var passwordHash, provider, googleIDVal, name, avatarURL, stripeCustomerID, stripeConnectID sql.NullString
	var updatedAt sql.NullTime
	err := r.db.QueryRow(`
		SELECT id, email, password_hash, provider, google_id,
			name, avatar_url, plan, stripe_customer_id, stripe_connect_id, next_invoice_seq, next_estimate_seq, language, created_at, updated_at
		FROM users WHERE google_id = $1
	`, googleID).Scan(
		&u.ID, &u.Email, &passwordHash, &provider, &googleIDVal,
		&name, &avatarURL, &u.Plan, &stripeCustomerID, &stripeConnectID, &u.NextInvoiceSeq, &u.NextEstimateSeq, &u.Language, &u.CreatedAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if passwordHash.Valid { u.PasswordHash = passwordHash.String }
	if provider.Valid { u.Provider = provider.String }
	if googleIDVal.Valid { u.GoogleID = googleIDVal.String }
	if name.Valid { u.Name = name.String }
	if avatarURL.Valid { u.AvatarURL = avatarURL.String }
	if stripeConnectID.Valid { u.StripeConnectID = stripeConnectID.String }
	if stripeCustomerID.Valid { u.StripeCustomerID = stripeCustomerID.String }
	if updatedAt.Valid { u.UpdatedAt = updatedAt.Time }
	return &u, nil
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
		SET google_id  = $1,
		    provider   = 'google',
		    updated_at = NOW()
		WHERE id = $2
		AND (google_id IS NULL OR google_id = $1)
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
