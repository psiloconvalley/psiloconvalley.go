// internal/repo/usage_repo.go
// Monthly usage tracking for plan limits.
// One row per user per month. Counters increment on each action.
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// UsageRepo tracks metered feature usage per user per month.
type UsageRepo struct {
	db *sql.DB
}

func NewUsageRepo(db *sql.DB) *UsageRepo {
	return &UsageRepo{db: db}
}

// currentMonth returns the year-month string for now, e.g. "2025-06".
func currentMonth() string {
	return time.Now().UTC().Format("2006-01")
}

// Increment bumps a counter for the current month.
// Valid columns: "invoices", "sends", "estimates", "reports".
func (r *UsageRepo) Increment(ctx context.Context, userID int64, column string) error {
	// Whitelist columns to prevent SQL injection (Rule 19)
	allowed := map[string]bool{
		"invoices":  true,
		"sends":     true,
		"estimates": true,
		"reports":   true,
	}
	if !allowed[column] {
		return fmt.Errorf("usage: invalid column %q", column)
	}

	ym := currentMonth()
	query := fmt.Sprintf(`
		INSERT INTO monthly_usage (user_id, year_month, %s)
		VALUES ($1, $2, 1)
		ON CONFLICT (user_id, year_month)
		DO UPDATE SET %s = monthly_usage.%s + 1, updated_at = now()
	`, column, column, column)

	_, err := r.db.ExecContext(ctx, query, userID, ym)
	return err
}

// Get returns the current month's count for a specific column.
// Returns 0 if no row exists (user hasn't used anything this month).
func (r *UsageRepo) Get(ctx context.Context, userID int64, column string) (int, error) {
	allowed := map[string]bool{
		"invoices":  true,
		"sends":     true,
		"estimates": true,
		"reports":   true,
	}
	if !allowed[column] {
		return 0, fmt.Errorf("usage: invalid column %q", column)
	}

	ym := currentMonth()
	query := fmt.Sprintf(`
		SELECT %s FROM monthly_usage
		WHERE user_id = $1 AND year_month = $2
	`, column)

	var count int
	err := r.db.QueryRowContext(ctx, query, userID, ym).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return count, nil
}
