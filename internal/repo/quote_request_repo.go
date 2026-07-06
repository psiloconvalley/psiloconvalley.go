// internal/repo/quote_request_repo.go
// Stores and retrieves quote requests from the public profile page.
package repo

import (
	"context"
	"database/sql"
	"fmt"
)

type QuoteRequestRepo struct {
	db *sql.DB
}

func NewQuoteRequestRepo(db *sql.DB) *QuoteRequestRepo {
	return &QuoteRequestRepo{db: db}
}

// Create inserts a new quote request from a public profile visitor.
func (r *QuoteRequestRepo) Create(ctx context.Context, qr *QuoteRequest) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO quote_requests
			(business_profile_id, client_name, client_phone, description, status)
		VALUES ($1, $2, $3, $4, 'new')
		RETURNING id
	`, qr.BusinessProfileID, qr.ClientName, qr.ClientPhone, qr.Description).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create quote request: %w", err)
	}
	return id, nil
}

// ListByUserID returns quote requests for a business owner's profile.
func (r *QuoteRequestRepo) ListByUserID(ctx context.Context, userID int64) ([]QuoteRequest, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT qr.id, qr.business_profile_id, qr.client_name,
		       qr.client_phone, qr.description, qr.status, qr.created_at
		FROM quote_requests qr
		INNER JOIN business_profiles bp ON bp.id = qr.business_profile_id
		WHERE bp.user_id = $1
		ORDER BY qr.created_at DESC
		LIMIT 50
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list quote requests: %w", err)
	}
	defer rows.Close()

	var requests []QuoteRequest
	for rows.Next() {
		var qr QuoteRequest
		if err := rows.Scan(
			&qr.ID, &qr.BusinessProfileID, &qr.ClientName,
			&qr.ClientPhone, &qr.Description, &qr.Status, &qr.CreatedAt,
		); err != nil {
			return nil, err
		}
		requests = append(requests, qr)
	}
	return requests, rows.Err()
}

// MarkViewed updates the status of a quote request to "viewed".
func (r *QuoteRequestRepo) MarkViewed(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE quote_requests SET status = 'viewed' WHERE id = $1 AND status = 'new'`,
		id,
	)
	return err
}
