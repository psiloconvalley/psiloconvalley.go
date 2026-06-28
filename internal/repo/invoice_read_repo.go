// internal/repo/invoice_read_repo.go
package repo

import (
	"context"
	"database/sql"
)

// GetInvoiceWithItems fetches a single invoice and its line items by ID.
// It LEFT JOINs business_profiles so the logo and company details are
// always populated from the canonical source — never stale invoice snapshots.
func (r *InvoiceRepo) GetInvoiceWithItems(
	ctx context.Context,
	id int64,
) (*Invoice, []InvoiceItem, error) {
	const q = `
		SELECT
			i.id,
			i.business_profile_id,
			i.client_id,
			i.user_id,
			i.anonymous_token,
			i.client_name,
			i.client_email,
			i.client_address,
			i.client_city,
			i.client_zip,
			i.client_state,
			i.client_country,
			COALESCE(bp.name,    i.company_name,    '') AS company_name,
			COALESCE(bp.email,   i.company_email,   '') AS company_email,
			COALESCE(bp.phone,   '')                AS company_phone,
			COALESCE(bp.address, i.company_address, '') AS company_address,
			COALESCE(bp.city,    i.company_city,    '') AS company_city,
			COALESCE(bp.zip,     i.company_zip,     '') AS company_zip,
			COALESCE(bp.state,   i.company_state,   '') AS company_state,
			COALESCE(bp.country, i.company_country, '') AS company_country,
			i.invoice_number,
			i.issue_date,
			i.due_date,
			i.tax_rate_bps,
			i.discount_amount_cents,
			i.notes,
			i.payment_details,
			i.subtotal_cents,
			i.tax_amount_cents,
			i.total_cents,
			i.currency,
			i.status,
			i.document_type,
			i.show_logo,
			i.show_title,
			i.auto_reminders,
			i.created_at,
			i.updated_at,
			COALESCE(bp.logo_url, '') AS logo_url,
			i.template_id,
			i.brand_color,
			COALESCE(i.logo_position, 'left') AS logo_position,
			COALESCE(i.public_token, '') AS public_token
		FROM invoices i
		LEFT JOIN business_profiles bp ON bp.id = i.business_profile_id
		WHERE i.id = $1`

	var inv Invoice
	var bpID, cID, uID sql.NullInt64
	var anonToken sql.NullString
	var dueDate sql.NullTime
	var updatedAt sql.NullTime
	var cEmail, cAddr, cCity, cZip, cState, cCountry sql.NullString
	var currency sql.NullString

	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&inv.ID,
		&bpID,
		&cID,
		&uID,
		&anonToken,
		&inv.ClientName,
		&cEmail,
		&cAddr,
		&cCity,
		&cZip,
		&cState,
		&cCountry,
		&inv.CompanyName,
		&inv.CompanyEmail,
		&inv.CompanyPhone,
		&inv.CompanyAddress,
		&inv.CompanyCity,
		&inv.CompanyZip,
		&inv.CompanyState,
		&inv.CompanyCountry,
		&inv.InvoiceNumber,
		&inv.IssueDate,
		&dueDate,
		&inv.TaxRateBps,
		&inv.DiscountAmountCents,
		&inv.Notes,
		&inv.PaymentDetails,
		&inv.SubtotalCents,
		&inv.TaxAmountCents,
		&inv.TotalCents,
		&currency,
		&inv.Status,
		&inv.DocumentType,
		&inv.ShowLogo,
		&inv.ShowTitle,
		&inv.AutoReminders,
		&inv.CreatedAt,
		&updatedAt,
		&inv.LogoURL,
		&inv.TemplateID,
		&inv.BrandColor,
		&inv.LogoPosition,
		&inv.PublicToken,
	)
	if err != nil {
		return nil, nil, err
	}

	if bpID.Valid {
		inv.BusinessProfileID = &bpID.Int64
	}
	if cID.Valid {
		inv.ClientID = &cID.Int64
	}
	if uID.Valid {
		inv.UserID = &uID.Int64
	}
	if anonToken.Valid && anonToken.String != "" {
		inv.AnonymousToken = anonToken.String
	}
	if dueDate.Valid {
		inv.DueDate = &dueDate.Time
	}
	if updatedAt.Valid {
		inv.UpdatedAt = updatedAt.Time
	}
	if cEmail.Valid {
		inv.ClientEmail = cEmail.String
	}
	if cAddr.Valid {
		inv.ClientAddress = cAddr.String
	}
	if cCity.Valid {
		inv.ClientCity = cCity.String
	}
	if cZip.Valid {
		inv.ClientZip = cZip.String
	}
	if cState.Valid {
		inv.ClientState = cState.String
	}
	if cCountry.Valid {
		inv.ClientCountry = cCountry.String
	}
	if currency.Valid {
		inv.Currency = currency.String
	}

	// ── Line items ────────────────────────────────────────────────────
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, invoice_id, description, details, quantity,
		       unit_price_cents, line_total_cents
		FROM invoice_items
		WHERE invoice_id = $1
		ORDER BY id`, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var items []InvoiceItem
	for rows.Next() {
		var it InvoiceItem
		var details sql.NullString
		if err := rows.Scan(
			&it.ID, &it.InvoiceID, &it.Description, &details,
			&it.Quantity, &it.UnitPriceCents, &it.LineTotalCents,
		); err != nil {
			return nil, nil, err
		}
		if details.Valid {
			it.Details = details.String
		}
		items = append(items, it)
	}
	return &inv, items, rows.Err()
}

// ListInvoices returns invoices for a user or all invoices for admin queries.
func (r *InvoiceRepo) ListInvoices(
	ctx context.Context,
	limit, offset int,
	userID *int64,
) ([]Invoice, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	q := `SELECT id, user_id, client_name, invoice_number,
	        issue_date, due_date, tax_rate_bps,
	        subtotal_cents, total_cents, currency, status, document_type, created_at
	      FROM invoices `

	var args []any
	if userID != nil {
		q += `WHERE user_id = $1 AND document_type = 'invoice' ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		args = append(args, *userID, limit, offset)
	} else {
		q += `WHERE document_type = 'invoice' ORDER BY created_at DESC LIMIT $1 OFFSET $2`
		args = append(args, limit, offset)
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invoices []Invoice
	for rows.Next() {
		var inv Invoice
		var uID sql.NullInt64
		var dueDate sql.NullTime
		var currency sql.NullString
		if err := rows.Scan(
			&inv.ID, &uID, &inv.ClientName, &inv.InvoiceNumber,
			&inv.IssueDate, &dueDate, &inv.TaxRateBps,
			&inv.SubtotalCents, &inv.TotalCents, &currency,
			&inv.Status, &inv.DocumentType, &inv.CreatedAt,
		); err != nil {
			return nil, err
		}
		if uID.Valid {
			inv.UserID = &uID.Int64
		}
		if dueDate.Valid {
			inv.DueDate = &dueDate.Time
		}
		if currency.Valid {
			inv.Currency = currency.String
		}
		invoices = append(invoices, inv)
	}
	return invoices, rows.Err()
}

// ListEstimates returns estimates for a user, ordered by most recent.
func (r *InvoiceRepo) ListEstimates(
	ctx context.Context,
	limit, offset int,
	userID int64,
) ([]Invoice, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	q := `SELECT id, user_id, client_name, invoice_number,
	             issue_date, due_date, tax_rate_bps,
	             subtotal_cents, total_cents, currency, status, document_type, created_at
	      FROM invoices
	      WHERE user_id = $1 AND document_type = 'estimate'
	      ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var estimates []Invoice
	for rows.Next() {
		var inv Invoice
		var uID sql.NullInt64
		var dueDate sql.NullTime
		var currency sql.NullString
		if err := rows.Scan(
			&inv.ID, &uID, &inv.ClientName, &inv.InvoiceNumber,
			&inv.IssueDate, &dueDate, &inv.TaxRateBps,
			&inv.SubtotalCents, &inv.TotalCents, &currency,
			&inv.Status, &inv.DocumentType, &inv.CreatedAt,
		); err != nil {
			return nil, err
		}
		if uID.Valid {
			inv.UserID = &uID.Int64
		}
		if dueDate.Valid {
			inv.DueDate = &dueDate.Time
		}
		if currency.Valid {
			inv.Currency = currency.String
		}
		estimates = append(estimates, inv)
	}
	return estimates, rows.Err()
}

// InvoiceNumberExists checks if an invoice number is already in use for a user.
func (r *InvoiceRepo) InvoiceNumberExists(
	ctx context.Context,
	number string,
	userID int64,
) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM invoices
			WHERE invoice_number = $1
			AND user_id = $2
		)`,
		number, userID,
	).Scan(&exists)
	return exists, err
}

// GetByPublicToken fetches an invoice by its public_token column.
// Used by the OG image handler — no auth required, token is the access control.
// Returns nil, nil if not found (caller should 404).
func (r *InvoiceRepo) GetByPublicToken(
	ctx context.Context,
	token string,
) (*Invoice, []InvoiceItem, error) {
	var id int64
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM invoices WHERE public_token = $1 LIMIT 1`,
		token,
	).Scan(&id)
	if err != nil {
		return nil, nil, nil // not found — caller serves 404
	}
	return r.GetInvoiceWithItems(ctx, id)
}
