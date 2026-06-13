// internal/repo/invoice_reports.go
package repo

import (
	"context"
	"database/sql"
	"time"
)

// ListInvoicesForReport returns invoices for a user within a date range.
// status = "all" returns every status. Any other value filters to that status.
func (r *InvoiceRepo) ListInvoicesForReport(
	ctx context.Context,
	userID int64,
	start, end time.Time,
	status string,
) ([]InvoiceReportRow, error) {
	const q = `
		SELECT
			COALESCE(invoice_number, ''),
			COALESCE(client_name, ''),
			COALESCE(client_email, ''),
			issue_date,
			due_date,
			status,
			subtotal_cents,
			COALESCE(tax_amount_cents, 0),
			total_cents,
			COALESCE(currency, 'USD'),
			updated_at
		FROM invoices
		WHERE user_id        = $1
		  AND document_type  = 'invoice'
		  AND issue_date    >= $2
		  AND issue_date    <= $3
		  AND ($4 = 'all' OR status = $4)
		ORDER BY issue_date DESC
	`

	rows, err := r.db.QueryContext(ctx, q, userID, start, end, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []InvoiceReportRow
	for rows.Next() {
		var row InvoiceReportRow
		var dueDate  sql.NullTime
		var updatedAt time.Time

		if err := rows.Scan(
			&row.InvoiceNumber,
			&row.ClientName,
			&row.ClientEmail,
			&row.IssueDate,
			&dueDate,
			&row.Status,
			&row.SubtotalCents,
			&row.TaxCents,
			&row.TotalCents,
			&row.Currency,
			&updatedAt,
		); err != nil {
			return nil, err
		}

		if dueDate.Valid {
			row.DueDate = &dueDate.Time
		}

		// DaysToPayment: only meaningful for paid invoices.
		// updatedAt is our best proxy for when payment was recorded.
		if row.Status == "paid" {
			days := int(updatedAt.Sub(row.IssueDate).Hours() / 24)
			row.DaysToPayment = &days
		}

		results = append(results, row)
	}
	return results, rows.Err()
}

// GetClientScorecards returns payment reliability data per client.
func (r *InvoiceRepo) GetClientScorecards(ctx context.Context, userID int64) ([]ClientScorecard, error) {
	const q = `
		SELECT
			COALESCE(client_name, 'Unknown'),
			COUNT(*),
			COALESCE(SUM(total_cents), 0),
			COALESCE(SUM(CASE WHEN status = 'paid' THEN total_cents ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status IN ('sent','overdue') THEN total_cents ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'overdue' THEN total_cents ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'paid' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'overdue' THEN 1 ELSE 0 END), 0),
			COALESCE(
				AVG(
					CASE WHEN status = 'paid' AND due_date IS NOT NULL
					THEN EXTRACT(EPOCH FROM (updated_at - issue_date)) / 86400
					ELSE NULL END
				), 0
			),
			CASE
				WHEN COUNT(*) FILTER (WHERE status IN ('paid','overdue')) = 0 THEN 0
				ELSE ROUND(
					100.0 * COUNT(*) FILTER (WHERE status = 'paid' AND due_date IS NOT NULL AND updated_at <= due_date)
					/ NULLIF(COUNT(*) FILTER (WHERE status IN ('paid','overdue')), 1)
				)
			END
		FROM invoices
		WHERE user_id = $1
		  AND document_type = 'invoice'
		  AND client_name IS NOT NULL
		  AND client_name != ''
		GROUP BY client_name
		ORDER BY SUM(total_cents) DESC
	`

	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ClientScorecard
	for rows.Next() {
		var cs ClientScorecard
		var avgDays float64
		var onTimeRate sql.NullFloat64

		if err := rows.Scan(
			&cs.ClientName,
			&cs.InvoiceCount,
			&cs.TotalBilled,
			&cs.TotalPaid,
			&cs.Outstanding,
			&cs.Overdue,
			&cs.PaidCount,
			&cs.OverdueCount,
			&avgDays,
			&onTimeRate,
		); err != nil {
			return nil, err
		}

		cs.AvgDaysToPayment = int(avgDays)
		if onTimeRate.Valid {
			cs.OnTimeRate = int(onTimeRate.Float64)
		}

		// ── Score algorithm (1-10) ──────────────────────────────
		// Weighted: on-time rate (50%), avg payment speed (30%), overdue ratio (20%)
		score := 5.0 // baseline

		// On-time component: 0-100% → 0-5 points
		score += float64(cs.OnTimeRate) / 100.0 * 5.0

		// Speed penalty: >30 days avg → lose points
		if cs.AvgDaysToPayment > 30 {
			score -= 2.0
		} else if cs.AvgDaysToPayment > 14 {
			score -= 1.0
		} else if cs.AvgDaysToPayment <= 7 && cs.PaidCount > 0 {
			score += 1.0
		}

		// Overdue penalty
		if cs.OverdueCount > 0 {
			ratio := float64(cs.OverdueCount) / float64(cs.InvoiceCount)
			score -= ratio * 3.0
		}

		// Clamp 1-10
		if score > 10 {
			score = 10
		}
		if score < 1 {
			score = 1
		}
		cs.Score = int(score)

		results = append(results, cs)
	}
	return results, rows.Err()
}
