9func (r *InvoiceRepo) GetInvoiceWithItems(
	ctx context.Context,
	id int64,
) (*Invoice, []InvoiceItem, error) {
	const q = `
		SELECT
			i.id, i.business_profile_id, i.client_id, i.user_id, i.anonymous_token,
			i.client_name, i.client_email, i.client_address,
			i.client_city, i.client_zip, i.client_state, i.client_country,
			i.invoice_number, i.issue_date, i.due_date,
			i.tax_rate_bps, i.discount_amount_cents, i.notes, i.payment_details,
			i.subtotal_cents, i.tax_amount_cents, i.total_cents,
			i.currency, i.status, i.created_at, i.updated_at,
			i.logo_url,
			COALESCE(bp.logo_url, i.logo_url) as effective_logo_url,
			COALESCE(bp.name, i.company_name) as company_name,
			COALESCE(bp.email, i.company_email) as company_email,
			COALESCE(bp.address, i.company_address) as company_address,
			COALESCE(bp.city, i.company_city) as company_city,
			COALESCE(bp.state, i.company_state) as company_state,
			COALESCE(bp.zip, i.company_zip) as company_zip,
			COALESCE(bp.country, i.company_country) as company_country
		FROM invoices i
		LEFT JOIN business_profiles bp ON bp.id = i.business_profile_id
		WHERE i.id = $1`

	var inv Invoice
	var anonToken sql.NullString
	var dueDate sql.NullTime
	var updatedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&inv.ID, &inv.BusinessProfileID, &inv.ClientID, &inv.UserID, &anonToken,
		&inv.ClientName, &inv.ClientEmail, &inv.ClientAddress,
		&inv.ClientCity, &inv.ClientZip, &inv.ClientState, &inv.ClientCountry,
		&inv.InvoiceNumber, &inv.IssueDate, &dueDate,
		&inv.TaxRateBps, &inv.DiscountAmountCents, &inv.Notes, &inv.PaymentDetails,
		&inv.SubtotalCents, &inv.TaxAmountCents, &inv.TotalCents,
		&inv.Currency, &inv.Status, &inv.CreatedAt, &updatedAt,
		&inv.LogoURL,
		&inv.LogoURL,
		&inv.CompanyName,
		&inv.CompanyEmail,
		&inv.CompanyAddress,
		&inv.CompanyCity,
		&inv.CompanyState,
		&inv.CompanyZip,
		&inv.CompanyCountry,
	)
	if err != nil {
		return nil, nil, err
	}

	if dueDate.Valid {
		inv.DueDate = &dueDate.Time
	}
	if updatedAt.Valid {
		inv.UpdatedAt = updatedAt.Time
	}
	if anonToken.Valid && anonToken.String != "" {
		inv.AnonymousToken = anonToken.String
	}

	// Items
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, description, details, quantity, unit_price_cents, line_total_cents
		FROM invoice_items WHERE invoice_id = $1 ORDER BY id`, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var items []InvoiceItem
	for rows.Next() {
		var it InvoiceItem
		var details sql.NullString
		if err := rows.Scan(&it.ID, &it.Description, &details, &it.Quantity, &it.UnitPriceCents, &it.LineTotalCents); err != nil {
			return nil, nil, err
		}
		if details.Valid {
			it.Details = details.String
		}
		items = append(items, it)
	}

	return &inv, items, rows.Err()
}
