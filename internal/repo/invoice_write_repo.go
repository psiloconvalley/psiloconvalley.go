// internal/repo/invoice_write_repo.go
package repo

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

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
			logo_position=$33, document_type=$34, updated_at=NOW()
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
	paymentMethod string,
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
		`UPDATE invoices 
		 SET status = $1, payment_method = $2, updated_at = NOW()
		 WHERE id = $3 AND user_id = $4`,
		newStatus, paymentMethod, id, userID,
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
