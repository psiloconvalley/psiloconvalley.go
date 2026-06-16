// internal/repo/invoice_repo.go
package repo

import (
	"context"
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

