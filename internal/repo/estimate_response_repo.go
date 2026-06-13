// internal/repo/estimate_response_repo.go
package repo

import (
	"context"
	"database/sql"
)

type EstimateResponseRepo struct{ db *sql.DB }

func NewEstimateResponseRepo(db *sql.DB) *EstimateResponseRepo {
	return &EstimateResponseRepo{db: db}
}

func (r *EstimateResponseRepo) Create(ctx context.Context, resp *EstimateResponse) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO estimate_responses (estimate_id, action, message, client_name)
		VALUES ($1, $2, $3, $4)
	`, resp.EstimateID, resp.Action, resp.Message, resp.ClientName)
	return err
}

func (r *EstimateResponseRepo) ListByEstimateID(ctx context.Context, estimateID int64) ([]EstimateResponse, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, estimate_id, action, message, client_name, created_at
		FROM estimate_responses
		WHERE estimate_id = $1
		ORDER BY created_at DESC
	`, estimateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var responses []EstimateResponse
	for rows.Next() {
		var r EstimateResponse
		var message, clientName sql.NullString
		if err := rows.Scan(&r.ID, &r.EstimateID, &r.Action, &message, &clientName, &r.CreatedAt); err != nil {
			return nil, err
		}
		if message.Valid {
			r.Message = message.String
		}
		if clientName.Valid {
			r.ClientName = clientName.String
		}
		responses = append(responses, r)
	}
	return responses, rows.Err()
}
