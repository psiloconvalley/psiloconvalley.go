// internal/repo/invoice_stats.go
package repo

import (
	"context"
	"database/sql"
)

// Dashboard Stats
func (r *InvoiceRepo) GetDashboardStats(ctx context.Context, userID int64) (*DashboardStats, error) {
	const q = `
		SELECT
			COALESCE(SUM(CASE WHEN status = 'paid' THEN total_cents ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'sent' THEN total_cents ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'overdue' THEN total_cents ELSE 0 END), 0),
			COUNT(CASE WHEN created_at >= date_trunc('month', NOW()) THEN 1 END),
			COUNT(*)
		FROM invoices
		WHERE user_id = $1`

	var s DashboardStats
	err := r.db.QueryRowContext(ctx, q, userID).Scan(
		&s.RevenueCents,
		&s.OutstandingCents,
		&s.OverdueCents,
		&s.MonthlyCount,
		&s.TotalCount,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *InvoiceRepo) GetAdminStats(ctx context.Context, db *sql.DB) (*AdminStats, error) {
	var s AdminStats

	// User counts
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&s.TotalUsers)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE created_at >= NOW() - INTERVAL '7 days'`).Scan(&s.NewUsersThisWeek)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE plan = 'pro'`).Scan(&s.ProUsers)

	// Invoice counts
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoices WHERE document_type = 'invoice'`).Scan(&s.TotalInvoices)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoices WHERE document_type = 'invoice' AND created_at >= date_trunc('month', NOW())`).Scan(&s.MonthlyInvoices)

	// Estimate counts
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoices WHERE document_type = 'estimate'`).Scan(&s.TotalEstimates)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoices WHERE document_type = 'estimate' AND status = 'sent'`).Scan(&s.EstimatesSent)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoices WHERE document_type = 'estimate' AND status = 'accepted'`).Scan(&s.EstimatesAccepted)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoices WHERE document_type = 'estimate' AND status = 'declined'`).Scan(&s.EstimatesDeclined)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoices WHERE document_type = 'estimate' AND status = 'converted'`).Scan(&s.EstimatesConverted)

	// Revenue
	_ = db.QueryRowContext(ctx, `SELECT COALESCE(SUM(total_cents), 0) FROM invoices WHERE document_type = 'invoice' AND status = 'paid'`).Scan(&s.TotalRevenueCents)
	_ = db.QueryRowContext(ctx, `SELECT COALESCE(SUM(total_cents), 0) FROM invoices WHERE document_type = 'invoice' AND status IN ('sent', 'overdue')`).Scan(&s.TotalOutstandingCents)

	return &s, nil
}
