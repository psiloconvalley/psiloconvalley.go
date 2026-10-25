// internal/repo/invoice_view_repo.go
package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// dbExecutor is the subset of *sql.DB methods this repo uses.
// Both *sql.DB and *sql.Tx satisfy it, which lets tests run each case
// inside a transaction and roll back for isolation.
type dbExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// InvoiceView is one logged view of an invoice by an external viewer.
// Owner views (authenticated or anon-cookie match) are never logged, so
// there is no viewer_type for "owner" — only "public_link" and "bot".
type InvoiceView struct {
	ID         int64
	InvoiceID  int64
	EventType  string // "html" | "pdf" | "pay"
	ViewerType string // "public_link" | "bot"
	ViewedAt   time.Time
	IPAddress  *string // nullable — INET column
	UserAgent  *string
	Referer    *string
}

// InvoiceViewRepo persists and queries invoice view events.
type InvoiceViewRepo struct{ db dbExecutor }

// NewInvoiceViewRepo constructs a repo bound to a real database pool.
// Tests can construct directly with a *sql.Tx via the unexported field.
func NewInvoiceViewRepo(db *sql.DB) *InvoiceViewRepo {
	return &InvoiceViewRepo{db: db}
}

// Log inserts a view row. Caller is responsible for having run DecideView
// first — this method does not re-check whether logging should happen.
//
// viewed_at is set by the database default (now()) and is not supplied here.
func (r *InvoiceViewRepo) Log(ctx context.Context, v InvoiceView) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO invoice_views
			(invoice_id, event_type, viewer_type, ip_address, user_agent, referer)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		v.InvoiceID,
		v.EventType,
		v.ViewerType,
		v.IPAddress,
		v.UserAgent,
		v.Referer,
	)
	return err
}

// LastHumanViewFor returns the most recent public_link view for an invoice,
// or (nil, nil) if there is none. Bot rows are excluded.
func (r *InvoiceViewRepo) LastHumanViewFor(
	ctx context.Context,
	invoiceID int64,
) (*InvoiceView, error) {
	var v InvoiceView
	var ip, ua, ref sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT id, invoice_id, event_type, viewer_type, viewed_at,
		       ip_address::text, user_agent, referer
		  FROM invoice_views
		 WHERE invoice_id = $1 AND viewer_type = 'public_link'
		 ORDER BY viewed_at DESC
		 LIMIT 1
	`, invoiceID).Scan(
		&v.ID,
		&v.InvoiceID,
		&v.EventType,
		&v.ViewerType,
		&v.ViewedAt,
		&ip,
		&ua,
		&ref,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if ip.Valid {
		v.IPAddress = &ip.String
	}
	if ua.Valid {
		v.UserAgent = &ua.String
	}
	if ref.Valid {
		v.Referer = &ref.String
	}
	return &v, nil
}

// LastHumanViewForInvoices returns the most recent public_link view per
// invoice ID. Invoices with no public_link views are absent from the map.
// Bot rows are excluded.
//
// This is a single round-trip batch query — avoids N+1 when rendering
// invoice lists.
func (r *InvoiceViewRepo) LastHumanViewForInvoices(
	ctx context.Context,
	ids []int64,
) (map[int64]InvoiceView, error) {
	result := make(map[int64]InvoiceView)
	if len(ids) == 0 {
		return result, nil
	}

	// DISTINCT ON (invoice_id) with ORDER BY (invoice_id, viewed_at DESC)
	// gives us the newest public_link row per invoice in one query.
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT ON (invoice_id)
		       id, invoice_id, event_type, viewer_type, viewed_at,
		       ip_address::text, user_agent, referer
		  FROM invoice_views
		 WHERE invoice_id = ANY($1) AND viewer_type = 'public_link'
		 ORDER BY invoice_id, viewed_at DESC
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var v InvoiceView
		var ip, ua, ref sql.NullString
		if err := rows.Scan(
			&v.ID,
			&v.InvoiceID,
			&v.EventType,
			&v.ViewerType,
			&v.ViewedAt,
			&ip,
			&ua,
			&ref,
		); err != nil {
			return nil, err
		}
		if ip.Valid {
			v.IPAddress = &ip.String
		}
		if ua.Valid {
			v.UserAgent = &ua.String
		}
		if ref.Valid {
			v.Referer = &ref.String
		}
		result[v.InvoiceID] = v
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
