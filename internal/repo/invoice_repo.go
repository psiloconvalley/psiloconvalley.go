// internal/repo/invoice_repo.go
package repo

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"math"
	"fmt"
	"time"
)


// CreateWithToken creates a freemium invoice for anon or logged-in users.
// FIX: For logged-in users, we look up their business profile and link
// it to the invoice so logo and company details appear correctly.
func (r *InvoiceRepo) CreateWithToken(
	ctx context.Context,
	user *User,
	anonymousToken string,
	clientName string,
	amount float64,
	description string,
) (int64, error) {
	var userID *int64
	var bizProfileID *int64

	if user != nil {
		userID = &user.ID

		// Look up business profile so the invoice is linked
		// and the logo/company details flow through correctly.
		var bpID int64
		err := r.db.QueryRowContext(ctx,
			`SELECT id FROM business_profiles WHERE user_id = $1 LIMIT 1`,
			user.ID,
		).Scan(&bpID)
		if err == nil {
			bizProfileID = &bpID
		}
		// If no business profile yet, bizProfileID stays nil — that's fine.
	}

	inv := &Invoice{
		UserID:            userID,
		BusinessProfileID: bizProfileID,
		AnonymousToken:    anonymousToken,
		ClientName:        clientName,
		InvoiceNumber:     fmt.Sprintf("INV-%d", time.Now().UnixNano()),
		IssueDate:         time.Now(),
		Currency:          "USD",
		Status:            "draft",
		Notes:             description,
	}

	items := []InvoiceItem{
		{
			Description:    description,
			Quantity:       1,
			UnitPriceCents: int64(math.Round(amount * 100)),
			LineTotalCents: int64(math.Round(amount * 100)),
		},
	}

	return r.CreateInvoice(ctx, inv, items, anonymousToken)
}

func (r *InvoiceRepo) CreateInvoice(
	ctx context.Context,
	inv *Invoice,
	items []InvoiceItem,
	anonymousToken string,
) (int64, error) {
	if inv == nil || inv.InvoiceNumber == "" {
		return 0, errors.New("invoice and number required")
	}

	items = calculateTotals(inv, items)

	if inv.Status == "" {
		inv.Status = "draft"
	}
	if inv.IssueDate.IsZero() {
		inv.IssueDate = time.Now()
	}
	if inv.Currency == "" {
		inv.Currency = "USD"
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

		const insertInvoice = `
				INSERT INTO invoices (
			business_profile_id, client_id, user_id, anonymous_token,
			client_name, client_email, client_address,
			client_city, client_zip, client_state, client_country,
			company_name, company_email, company_address,
			company_city, company_zip, company_state, company_country,
			invoice_number, issue_date, due_date,
			tax_rate_bps, discount_amount_cents, notes, payment_details,
			subtotal_cents, tax_amount_cents, total_cents,
			currency, status, show_logo, show_title, auto_reminders,
			template_id, brand_color, logo_position, document_type
		) VALUES (
			$1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,$9,$10,$11,
			$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,
			$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,
			$34,$35,$36,$37
		) RETURNING id, created_at`

	var newID int64
	var createdAt time.Time

	if inv.DocumentType == "" {
		inv.DocumentType = "invoice"
	}

	err = tx.QueryRowContext(ctx, insertInvoice,
		inv.BusinessProfileID, inv.ClientID, inv.UserID, anonymousToken,
		inv.ClientName, inv.ClientEmail, inv.ClientAddress,
		inv.ClientCity, inv.ClientZip, inv.ClientState, inv.ClientCountry,
		inv.CompanyName, inv.CompanyEmail, inv.CompanyAddress,
		inv.CompanyCity, inv.CompanyZip, inv.CompanyState, inv.CompanyCountry,
		inv.InvoiceNumber, inv.IssueDate, inv.DueDate,
		inv.TaxRateBps, inv.DiscountAmountCents, inv.Notes, inv.PaymentDetails,
		inv.SubtotalCents, inv.TaxAmountCents, inv.TotalCents,
		inv.Currency, inv.Status, inv.ShowLogo, inv.ShowTitle, inv.AutoReminders,
		inv.TemplateID, inv.BrandColor, inv.LogoPosition, inv.DocumentType,
	).Scan(&newID, &createdAt)
		if err != nil {
		return 0, fmt.Errorf("insert invoice: %w", err)
	}

	inv.ID = newID
	inv.CreatedAt = createdAt

	const insertItem = `
		INSERT INTO invoice_items
			(invoice_id, description, details, quantity,
			 unit_price_cents, line_total_cents)
		VALUES ($1, $2, $3, $4, $5, $6)`

	for _, item := range items {
		if _, err := tx.ExecContext(ctx, insertItem,
			newID, item.Description, item.Details,
			item.Quantity, item.UnitPriceCents, item.LineTotalCents,
		); err != nil {
			return 0, fmt.Errorf("insert item: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return newID, nil
}

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

func (r *InvoiceRepo) UpdateInvoice(
	ctx context.Context,
	inv *Invoice,
	items []InvoiceItem,
) error {
	if inv.UserID == nil {
		return errors.New("UpdateInvoice: user_id is required")
	}

	items = calculateTotals(inv, items)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

		const uq = `
		UPDATE invoices SET
			client_id=$1, client_name=$2, client_email=$3, client_address=$4,
			client_city=$5, client_zip=$6, client_state=$7, client_country=$8,
			company_name=$9, company_email=$10, company_address=$11,
			company_city=$12, company_zip=$13, company_state=$14, company_country=$15,
			invoice_number=$16, issue_date=$17, due_date=$18,
			tax_rate_bps=$19, discount_amount_cents=$20, notes=$21, payment_details=$22,
			subtotal_cents=$23, tax_amount_cents=$24, total_cents=$25,
			currency=$26, status=$27, show_logo=$28, show_title=$29,
			auto_reminders=$30, template_id=$31, brand_color=$32,
			logo_position=$33,document_type=$34, updated_at=NOW()
			WHERE id=$35 AND user_id=$36`

	res, err := tx.ExecContext(ctx, uq,
		inv.ClientID, inv.ClientName, inv.ClientEmail, inv.ClientAddress,
		inv.ClientCity, inv.ClientZip, inv.ClientState, inv.ClientCountry,
		inv.CompanyName, inv.CompanyEmail, inv.CompanyAddress,
		inv.CompanyCity, inv.CompanyZip, inv.CompanyState, inv.CompanyCountry,
		inv.InvoiceNumber, inv.IssueDate, inv.DueDate,
		inv.TaxRateBps, inv.DiscountAmountCents, inv.Notes, inv.PaymentDetails,
		inv.SubtotalCents, inv.TaxAmountCents, inv.TotalCents,
		inv.Currency, inv.Status, inv.ShowLogo, inv.ShowTitle,
		inv.AutoReminders, inv.TemplateID, inv.BrandColor,		
		inv.LogoPosition, inv.DocumentType, inv.ID, inv.UserID,
	)
				if err != nil {

		return err
	}

	aff, _ := res.RowsAffected()
	if aff == 0 {
		return fmt.Errorf("invoice %d not found or access denied", inv.ID)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM invoice_items WHERE invoice_id = $1`, inv.ID,
	); err != nil {
		return fmt.Errorf("delete items: %w", err)
	}

	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO invoice_items
				(invoice_id, description, details, quantity,
				 unit_price_cents, line_total_cents)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			inv.ID, item.Description, item.Details,
			item.Quantity, item.UnitPriceCents, item.LineTotalCents,
		); err != nil {
			return fmt.Errorf("insert item: %w", err)
		}
	}

	return tx.Commit()
}

func (r *InvoiceRepo) UpdateInvoiceStatus(
	ctx context.Context,
	id int64,
	newStatus string,
	userID int64,
) error {
	validStatuses := map[string]bool{
		"draft": true, "sent": true, "paid": true,
		"void": true, "overdue": true,
	}
	if !validStatuses[newStatus] {
		return fmt.Errorf("invalid status: %s", newStatus)
	}

	var currentStatus string
	err := r.db.QueryRowContext(ctx,
		`SELECT status FROM invoices WHERE id = $1 AND user_id = $2`,
		id, userID,
	).Scan(&currentStatus)
	if err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}

	validTransitions := map[string]map[string]bool{
		"draft":   {"sent": true, "void": true},
		"sent":    {"paid": true, "overdue": true, "void": true},
		"overdue": {"paid": true, "void": true},
		"paid":    {},
		"void":    {},
	}

	allowed, ok := validTransitions[currentStatus]
	if !ok || !allowed[newStatus] {
		return fmt.Errorf("cannot transition from %s to %s", currentStatus, newStatus)
	}

	_, err = r.db.ExecContext(ctx,
		`UPDATE invoices SET status = $1, updated_at = NOW()
		 WHERE id = $2 AND user_id = $3`,
		newStatus, id, userID,
	)
	return err
}

func (r *InvoiceRepo) DeleteDraftInvoice(
	ctx context.Context,
	id int64,
	userID int64,
) error {
	var status string
	err := r.db.QueryRowContext(ctx,
		`SELECT status FROM invoices WHERE id = $1 AND user_id = $2`,
		id, userID,
	).Scan(&status)
	if err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}
	if status != "draft" {
		return fmt.Errorf(
			"only draft invoices can be deleted — use void for %s invoices",
			status,
		)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM invoice_items WHERE invoice_id = $1`, id,
	); err != nil {
		return fmt.Errorf("delete items: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM invoices WHERE id = $1 AND user_id = $2 AND status = 'draft'`,
		id, userID,
	); err != nil {
		return fmt.Errorf("delete invoice: %w", err)
	}

	return tx.Commit()
}

// EnsurePublicToken returns the existing public token for an invoice or
// generates and saves a new one if it does not yet exist.
func (r *InvoiceRepo) EnsurePublicToken(ctx context.Context, invoiceID int64) (string, error) {
	var existing sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT public_token FROM invoices WHERE id = $1`,
		invoiceID,
	).Scan(&existing)
	if err != nil {
		return "", err
	}
	if existing.Valid && existing.String != "" {
		return existing.String, nil
	}

	token, err := generatePublicToken()
	if err != nil {
		return "", err
	}

	_, err = r.db.ExecContext(ctx,
		`UPDATE invoices SET public_token = $1 WHERE id = $2`,
		token, invoiceID,
	)
	if err != nil {
		return "", err
	}
	return token, nil
}

// generatePublicToken creates a 32-byte hex token (64 chars).
func generatePublicToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
// UpdateEstimateStatus validates the transition and updates the estimate status.
// Estimate statuses and transitions are distinct from invoice statuses.
// This is the single source of truth for estimate lifecycle rules.
func (r *InvoiceRepo) UpdateEstimateStatus(
	ctx context.Context,
	id int64,
	userID int64,
	newStatus string,
) error {
	validStatuses := map[string]bool{
		"draft": true, "sent": true, "accepted": true,
		"rejected": true, "expired": true, "converted": true,
	}
	if !validStatuses[newStatus] {
		return fmt.Errorf("invalid estimate status: %s", newStatus)
	}

	var currentStatus string
	err := r.db.QueryRowContext(ctx,
		`SELECT status FROM invoices
		 WHERE id = $1 AND user_id = $2 AND document_type = 'estimate'`,
		id, userID,
	).Scan(&currentStatus)
	if err != nil {
		return fmt.Errorf("estimate not found: %w", err)
	}

	validTransitions := map[string]map[string]bool{
		"draft":     {"sent": true},
		"sent":      {"accepted": true, "rejected": true, "expired": true},
		"accepted":  {"converted": true},
		"rejected":  {},
		"expired":   {},
		"converted": {},
	}

	allowed, ok := validTransitions[currentStatus]
	if !ok || !allowed[newStatus] {
		return fmt.Errorf("cannot transition estimate from %s to %s", currentStatus, newStatus)
	}

	_, err = r.db.ExecContext(ctx,
		`UPDATE invoices SET status = $1, updated_at = NOW()
		 WHERE id = $2 AND user_id = $3`,
		newStatus, id, userID,
	)
	return err
}
