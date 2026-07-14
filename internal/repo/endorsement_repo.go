// internal/repo/endorsement_repo.go
// CRUD for business profile endorsements.
// Endorsements go live immediately on submission — no moderation queue.
// Owner can delete any endorsement. Token is the client's access key.
package repo

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

type EndorsementRepo struct {
	db *sql.DB
}

func NewEndorsementRepo(db *sql.DB) *EndorsementRepo {
	return &EndorsementRepo{db: db}
}

// generateToken creates a unique 32-char hex token — URL-safe, unguessable.
func generateEndorsementToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate endorsement token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// RequestEndorsement creates a pending endorsement for a paid invoice.
// Returns the token used to build the client-facing endorsement URL.
// Called when the owner taps "Request Endorsement" on a paid invoice.
func (r *EndorsementRepo) RequestEndorsement(
	ctx context.Context,
	bizProfileID int64,
	invoiceID *int64,
) (string, error) {
	token, err := generateEndorsementToken()
	if err != nil {
		return "", err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO endorsements
			(business_profile_id, invoice_id, token, status)
		VALUES ($1, $2, $3, 'pending')
	`, bizProfileID, invoiceID, token)
	if err != nil {
		return "", fmt.Errorf("request endorsement: %w", err)
	}
	return token, nil
}

// GetByToken fetches an endorsement by its unique token.
// Used when a client opens the endorsement link.
// Returns nil if not found.
func (r *EndorsementRepo) GetByToken(ctx context.Context, token string) (*Endorsement, error) {
	var e Endorsement
	var invoiceID sql.NullInt64
	var submittedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, `
		SELECT id, business_profile_id, invoice_id,
		       endorser_name, endorser_location,
		       rating, body, token, status,
		       requested_at, submitted_at
		FROM endorsements
		WHERE token = $1
		LIMIT 1
	`, token).Scan(
		&e.ID, &e.BusinessProfileID, &invoiceID,
		&e.EndorserName, &e.EndorserLocation,
		&e.Rating, &e.Body, &e.Token, &e.Status,
		&e.RequestedAt, &submittedAt,
	)
	if err != nil {
		return nil, err
	}
	if invoiceID.Valid {
		e.InvoiceID = &invoiceID.Int64
	}
	if submittedAt.Valid {
		e.SubmittedAt = &submittedAt.Time
	}
	return &e, nil
}

// Submit fills in the endorsement from the client and marks it submitted.
// Goes live immediately — status becomes "submitted".
// Only works on pending endorsements (prevents double-submission).
func (r *EndorsementRepo) Submit(
	ctx context.Context,
	token, name, location, body string,
	rating int,
) error {
	now := time.Now()
	result, err := r.db.ExecContext(ctx, `
		UPDATE endorsements
		SET endorser_name     = $1,
		    endorser_location = $2,
		    body              = $3,
		    rating            = $4,
		    status            = 'submitted',
		    submitted_at      = $5
		WHERE token  = $6
		  AND status = 'pending'
	`, name, location, body, rating, now, token)
	if err != nil {
		return fmt.Errorf("submit endorsement: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("endorsement not found or already submitted")
	}
	return nil
}

// Decline marks an endorsement as declined.
// Client chose "not at this time" — never shown publicly.
func (r *EndorsementRepo) Decline(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE endorsements SET status = 'declined' WHERE token = $1 AND status = 'pending'`,
		token,
	)
	return err
}

// ListByUserID returns all endorsements for a business owner.
// Used for the owner's endorsement management page.
func (r *EndorsementRepo) ListByUserID(ctx context.Context, userID int64) ([]Endorsement, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.id, e.business_profile_id, e.invoice_id,
		       e.endorser_name, e.endorser_location,
		       e.rating, e.body, e.token, e.status,
		       e.requested_at, e.submitted_at
		FROM endorsements e
		INNER JOIN business_profiles bp ON bp.id = e.business_profile_id
		WHERE bp.user_id = $1
		ORDER BY e.requested_at DESC
		LIMIT 100
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list endorsements: %w", err)
	}
	defer rows.Close()

	var endorsements []Endorsement
	for rows.Next() {
		var e Endorsement
		var invoiceID sql.NullInt64
		var submittedAt sql.NullTime
		if err := rows.Scan(
			&e.ID, &e.BusinessProfileID, &invoiceID,
			&e.EndorserName, &e.EndorserLocation,
			&e.Rating, &e.Body, &e.Token, &e.Status,
			&e.RequestedAt, &submittedAt,
		); err != nil {
			return nil, err
		}
		if invoiceID.Valid {
			e.InvoiceID = &invoiceID.Int64
		}
		if submittedAt.Valid {
			e.SubmittedAt = &submittedAt.Time
		}
		endorsements = append(endorsements, e)
	}
	return endorsements, rows.Err()
}

// ListSubmittedBySlug returns all submitted (public) endorsements for a profile.
// Used on the public /biz/{slug} page.
// Only returns endorsements with rating > 0 and status = submitted.
func (r *EndorsementRepo) ListSubmittedBySlug(ctx context.Context, slug string) ([]Endorsement, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.id, e.business_profile_id,
		       e.endorser_name, e.endorser_location,
		       e.rating, e.body, e.submitted_at
		FROM endorsements e
		INNER JOIN business_profiles bp ON bp.id = e.business_profile_id
		WHERE bp.slug   = $1
		  AND e.status  = 'submitted'
		  AND e.rating  > 0
		ORDER BY e.submitted_at DESC
		LIMIT 20
	`, slug)
	if err != nil {
		return nil, fmt.Errorf("list submitted endorsements: %w", err)
	}
	defer rows.Close()

	var endorsements []Endorsement
	for rows.Next() {
		var e Endorsement
		var submittedAt sql.NullTime
		if err := rows.Scan(
			&e.ID, &e.BusinessProfileID,
			&e.EndorserName, &e.EndorserLocation,
			&e.Rating, &e.Body, &submittedAt,
		); err != nil {
			return nil, err
		}
		if submittedAt.Valid {
			e.SubmittedAt = &submittedAt.Time
		}
		endorsements = append(endorsements, e)
	}
	return endorsements, rows.Err()
}

// Delete permanently removes an endorsement.
// Ownership verified by joining through business_profiles.
// Owner safety valve — no soft delete needed.
func (r *EndorsementRepo) Delete(ctx context.Context, id, userID int64) error {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM endorsements
		USING business_profiles bp
		WHERE endorsements.id = $1
		  AND endorsements.business_profile_id = bp.id
		  AND bp.user_id = $2
	`, id, userID)
	if err != nil {
		return fmt.Errorf("delete endorsement: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("endorsement not found or not owned by user")
	}
	return nil
}

// AverageRating returns the average rating and count for a business profile.
// Used on the public profile page to show "⭐ 4.9 avg · 12 endorsements".
func (r *EndorsementRepo) AverageRating(ctx context.Context, bizProfileID int64) (avg float64, count int, err error) {
	err = r.db.QueryRowContext(ctx, `
		SELECT COALESCE(AVG(rating)::numeric(3,1), 0),
		       COUNT(*)
		FROM endorsements
		WHERE business_profile_id = $1
		  AND status = 'submitted'
		  AND rating > 0
	`, bizProfileID).Scan(&avg, &count)
	if err != nil {
		return 0, 0, fmt.Errorf("average rating: %w", err)
	}
	return avg, count, nil
}

// GetByInvoiceID fetches the endorsement for a specific invoice.
// Returns nil if none exists. Used for duplicate prevention.
func (r *EndorsementRepo) GetByInvoiceID(ctx context.Context, invoiceID int64) (*Endorsement, error) {
	var e Endorsement
	var invID sql.NullInt64
	var submittedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, `
		SELECT id, business_profile_id, invoice_id,
		       endorser_name, endorser_location,
		       rating, body, token, status,
		       requested_at, submitted_at
		FROM endorsements
		WHERE invoice_id = $1
		ORDER BY requested_at DESC
		LIMIT 1
	`, invoiceID).Scan(
		&e.ID, &e.BusinessProfileID, &invID,
		&e.EndorserName, &e.EndorserLocation,
		&e.Rating, &e.Body, &e.Token, &e.Status,
		&e.RequestedAt, &submittedAt,
	)
	if err != nil {
		return nil, err
	}
	if invID.Valid {
		e.InvoiceID = &invID.Int64
	}
	if submittedAt.Valid {
		e.SubmittedAt = &submittedAt.Time
	}
	return &e, nil
}
