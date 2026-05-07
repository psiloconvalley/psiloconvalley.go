package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"
)

// =====================================================================
// Domain Models
// =====================================================================

// BusinessProfile represents the issuer of invoices.
//
// This is the "live" company information. Invoices reference this
// via business_profile_id, but invoices are intentionally lean and
// do not currently store snapshot copies of company info.
type BusinessProfile struct {
	ID             int64
	Name           string
	Email          string
	Address        string
	TaxID          string
	DefaultTaxRate float64
	Currency       string
	LogoURL        string
	CreatedAt      time.Time
}

// Client represents a customer that may receive invoices.
type Client struct {
	ID                int64
	BusinessProfileID int64
	Name              string
	Email             string
	Address           string
	DefaultTaxRate    float64
	PaymentTerms      string
	CreatedAt         time.Time
}

// Invoice represents a single invoice document.
//
// Fields are snapshotted at invoice creation time so that edits to
// a client or business profile never mutate historical invoices.
// This is the correct model for any legal financial document.
type Invoice struct {
	ID                int64
	BusinessProfileID *int64
	ClientID          *int64

	// ── CHANGE 1: Added snapshot fields ──────────────────────────────
	// These fields exist in the database but were missing from the
	// struct. Without them, the edit form cannot pre-populate and
	// CreateInvoice() silently discards whatever the user typed.
	//
	// Company snapshot (who is sending the invoice)
	CompanyName    string
	CompanyEmail   string
	CompanyAddress string
	CompanyCity    string
	CompanyZip     string
	CompanyState   string
	CompanyCountry string

	// Client snapshot (who is receiving the invoice)
	ClientName    string
	ClientEmail   string
	ClientAddress string
	ClientCity    string
	ClientZip     string
	ClientState   string
	ClientCountry string

	// Invoice identity and timing
	InvoiceNumber string
	IssueDate     time.Time
	DueDate       *time.Time

	// Financials
	TaxRate        float64
	DiscountAmount float64
	Notes          string
	PaymentDetails string
	Subtotal       float64
	TaxAmount      float64
	Total          float64
	Currency       string

	// Metadata
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
	// ── END CHANGE 1 ─────────────────────────────────────────────────
}

// InvoiceItem represents a single line item on an invoice.
type InvoiceItem struct {
	ID          int64
	InvoiceID   int64
	Description string
	Quantity    float64
	UnitPrice   float64
	LineTotal   float64
}

// =====================================================================
// Repositories
// =====================================================================

// InvoiceRepo handles persistence for invoices and their items.
type InvoiceRepo struct {
	db *sql.DB
}

// NewInvoiceRepo constructs a new InvoiceRepo.
func NewInvoiceRepo(db *sql.DB) *InvoiceRepo {
	return &InvoiceRepo{db: db}
}

// ClientRepo handles persistence for clients.
type ClientRepo struct {
	db *sql.DB
}

// NewClientRepo constructs a new ClientRepo.
func NewClientRepo(db *sql.DB) *ClientRepo {
	return &ClientRepo{db: db}
}

// BusinessRepo handles persistence for business profiles.
type BusinessRepo struct {
	db *sql.DB
}

// NewBusinessRepo constructs a new BusinessRepo.
func NewBusinessRepo(db *sql.DB) *BusinessRepo {
	return &BusinessRepo{db: db}
}

// =====================================================================
// Internal helpers
// =====================================================================

// roundTwo rounds money values to two decimal places.
func roundTwo(v float64) float64 {
	return math.Round(v*100) / 100
}

// =====================================================================
// Invoice persistence
// =====================================================================

// CreateInvoice inserts a new invoice and its line items inside a
// single transaction. Totals are computed authoritatively in Go,
// not in PostgreSQL, so currency handling, discounts, and tax rules
// can evolve in the application layer.
func (r *InvoiceRepo) CreateInvoice(inv *Invoice, items []InvoiceItem) (int64, error) {

	if inv == nil {
		return 0, errors.New("invoice is required")
	}

	if inv.InvoiceNumber == "" {
		return 0, errors.New("invoice number is required")
	}

	// Compute totals authoritatively
	var subtotal float64
	for i := range items {
		line := roundTwo(items[i].Quantity * items[i].UnitPrice)
		items[i].LineTotal = line
		subtotal += line
	}
	subtotal = roundTwo(subtotal)

	taxAmount := roundTwo(subtotal * (inv.TaxRate / 100.0))
	total := roundTwo(subtotal + taxAmount)

	inv.Subtotal = subtotal
	inv.TaxAmount = taxAmount
	inv.Total = total

	if inv.Status == "" {
		inv.Status = "draft"
	}

	if inv.IssueDate.IsZero() {
		inv.IssueDate = time.Now()
	}

	if inv.Currency == "" {
		inv.Currency = "USD"
	}

	tx, err := r.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// ── CHANGE 1 (continued): INSERT now includes all snapshot fields ─
	const insertInvoice = `
		INSERT INTO invoices (
			business_profile_id,
			client_id,
			client_name,
			client_email,
			client_address,
			client_city,
			client_zip,
			client_state,
			client_country,
			company_name,
			company_email,
			company_address,
			company_city,
			company_zip,
			company_state,
			company_country,
			invoice_number,
			issue_date,
			due_date,
			tax_rate,
			notes,
			payment_details,
			subtotal,
			tax_amount,
			total,
			currency,
			status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27
		)
		RETURNING id, created_at
	`

	var newID int64
	var createdAt time.Time

	err = tx.QueryRow(
		insertInvoice,
		inv.BusinessProfileID, // $1
		inv.ClientID,          // $2
		inv.ClientName,        // $3
		inv.ClientEmail,       // $4
		inv.ClientAddress,     // $5
		inv.ClientCity,        // $6
		inv.ClientZip,         // $7
		inv.ClientState,       // $8
		inv.ClientCountry,     // $9
		inv.CompanyName,       // $10
		inv.CompanyEmail,      // $11
		inv.CompanyAddress,    // $12
		inv.CompanyCity,       // $13
		inv.CompanyZip,        // $14
		inv.CompanyState,      // $15
		inv.CompanyCountry,    // $16
		inv.InvoiceNumber,     // $17
		inv.IssueDate,         // $18
		inv.DueDate,           // $19
		inv.TaxRate,           // $20
		inv.Notes,             // $21
		inv.PaymentDetails,    // $22
		inv.Subtotal,          // $23
		inv.TaxAmount,         // $24
		inv.Total,             // $25
		inv.Currency,          // $26
		inv.Status,            // $27
	).Scan(&newID, &createdAt)
	if err != nil {
		return 0, fmt.Errorf("failed to insert invoice: %w", err)
	}

	inv.ID = newID
	inv.CreatedAt = createdAt

	const insertItem = `
		INSERT INTO invoice_items (
			invoice_id,
			description,
			quantity,
			unit_price,
			line_total
		) VALUES (
			$1, $2, $3, $4, $5
		)
	`

	for _, item := range items {
		_, err := tx.Exec(
			insertItem,
			newID,
			item.Description,
			item.Quantity,
			item.UnitPrice,
			item.LineTotal,
		)
		if err != nil {
			return 0, fmt.Errorf("failed to insert invoice item: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit invoice transaction: %w", err)
	}

	return newID, nil
}

// ── CHANGE 2: GetInvoiceWithItems now selects all snapshot fields ─────
//
// Previously only 15 columns were selected. The DB has 27+ columns.
// Company and client snapshot fields were never loaded into Go,
// meaning the edit form had nothing to pre-populate with.
func (r *InvoiceRepo) GetInvoiceWithItems(id int64) (*Invoice, []InvoiceItem, error) {

	const invoiceQuery = `
		SELECT
			id,
			business_profile_id,
			client_id,
			client_name,
			client_email,
			client_address,
			client_city,
			client_zip,
			client_state,
			client_country,
			company_name,
			company_email,
			company_address,
			company_city,
			company_zip,
			company_state,
			company_country,
			invoice_number,
			issue_date,
			due_date,
			tax_rate,
			notes,
			payment_details,
			subtotal,
			tax_amount,
			total,
			currency,
			status,
			created_at,
			updated_at
		FROM invoices
		WHERE id = $1
	`

	var inv Invoice

	// Nullable DB columns must be scanned into sql.Null* types
	// then converted to Go pointers after the scan completes.
	var (
		businessProfileID sql.NullInt64
		clientID          sql.NullInt64
		dueDate           sql.NullTime
		updatedAt         sql.NullTime

		// All snapshot string fields are nullable in the DB
		clientEmail    sql.NullString
		clientAddress  sql.NullString
		clientCity     sql.NullString
		clientZip      sql.NullString
		clientState    sql.NullString
		clientCountry  sql.NullString
		companyName    sql.NullString
		companyEmail   sql.NullString
		companyAddress sql.NullString
		companyCity    sql.NullString
		companyZip     sql.NullString
		companyState   sql.NullString
		companyCountry sql.NullString
		currency       sql.NullString
	)

	err := r.db.QueryRow(invoiceQuery, id).Scan(
		&inv.ID,             // id
		&businessProfileID,  // business_profile_id
		&clientID,           // client_id
		&inv.ClientName,     // client_name
		&clientEmail,        // client_email
		&clientAddress,      // client_address
		&clientCity,         // client_city
		&clientZip,          // client_zip
		&clientState,        // client_state
		&clientCountry,      // client_country
		&companyName,        // company_name
		&companyEmail,       // company_email
		&companyAddress,     // company_address
		&companyCity,        // company_city
		&companyZip,         // company_zip
		&companyState,       // company_state
		&companyCountry,     // company_country
		&inv.InvoiceNumber,  // invoice_number
		&inv.IssueDate,      // issue_date
		&dueDate,            // due_date
		&inv.TaxRate,        // tax_rate
		&inv.Notes,          // notes
		&inv.PaymentDetails, // payment_details
		&inv.Subtotal,       // subtotal
		&inv.TaxAmount,      // tax_amount
		&inv.Total,          // total
		&currency,           // currency
		&inv.Status,         // status
		&inv.CreatedAt,      // created_at
		&updatedAt,          // updated_at
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load invoice: %w", err)
	}

	// Convert sql.Null* → Go pointer/value types
	if businessProfileID.Valid {
		v := businessProfileID.Int64
		inv.BusinessProfileID = &v
	}
	if clientID.Valid {
		v := clientID.Int64
		inv.ClientID = &v
	}
	if dueDate.Valid {
		t := dueDate.Time
		inv.DueDate = &t
	}
	if updatedAt.Valid {
		inv.UpdatedAt = updatedAt.Time
	}

	// Snapshot string fields
	if clientEmail.Valid {
		inv.ClientEmail = clientEmail.String
	}
	if clientAddress.Valid {
		inv.ClientAddress = clientAddress.String
	}
	if clientCity.Valid {
		inv.ClientCity = clientCity.String
	}
	if clientZip.Valid {
		inv.ClientZip = clientZip.String
	}
	if clientState.Valid {
		inv.ClientState = clientState.String
	}
	if clientCountry.Valid {
		inv.ClientCountry = clientCountry.String
	}
	if companyName.Valid {
		inv.CompanyName = companyName.String
	}
	if companyEmail.Valid {
		inv.CompanyEmail = companyEmail.String
	}
	if companyAddress.Valid {
		inv.CompanyAddress = companyAddress.String
	}
	if companyCity.Valid {
		inv.CompanyCity = companyCity.String
	}
	if companyZip.Valid {
		inv.CompanyZip = companyZip.String
	}
	if companyState.Valid {
		inv.CompanyState = companyState.String
	}
	if companyCountry.Valid {
		inv.CompanyCountry = companyCountry.String
	}
	if currency.Valid {
		inv.Currency = currency.String
	}

	const itemsQuery = `
		SELECT
			id,
			invoice_id,
			description,
			quantity,
			unit_price,
			line_total
		FROM invoice_items
		WHERE invoice_id = $1
		ORDER BY id ASC
	`

	rows, err := r.db.Query(itemsQuery, id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load invoice items: %w", err)
	}
	defer rows.Close()

	var items []InvoiceItem

	for rows.Next() {
		var item InvoiceItem

		err := rows.Scan(
			&item.ID,
			&item.InvoiceID,
			&item.Description,
			&item.Quantity,
			&item.UnitPrice,
			&item.LineTotal,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to scan invoice item: %w", err)
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("invoice items iteration error: %w", err)
	}

	return &inv, items, nil
}

// ── END CHANGE 2 ──────────────────────────────────────────────────────

// ListInvoices returns the most recent invoices.
//
// Only columns that exist in the current database schema are selected.
// Nullable columns are read into sql.Null* and converted to pointers.
func (r *InvoiceRepo) ListInvoices(limit int, offset int) ([]Invoice, error) {

	const query = `
		SELECT
			id,
			business_profile_id,
			client_id,
			client_name,
			invoice_number,
			issue_date,
			due_date,
			tax_rate,
			notes,
			payment_details,
			subtotal,
			tax_amount,
			total,
			status,
			created_at
		FROM invoices
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list invoices: %w", err)
	}
	defer rows.Close()

	var invoices []Invoice

	for rows.Next() {

		var inv Invoice

		var (
			businessProfileID sql.NullInt64
			clientID          sql.NullInt64
			dueDate           sql.NullTime
		)

		err := rows.Scan(
			&inv.ID,
			&businessProfileID,
			&clientID,
			&inv.ClientName,
			&inv.InvoiceNumber,
			&inv.IssueDate,
			&dueDate,
			&inv.TaxRate,
			&inv.Notes,
			&inv.PaymentDetails,
			&inv.Subtotal,
			&inv.TaxAmount,
			&inv.Total,
			&inv.Status,
			&inv.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan invoice row: %w", err)
		}

		if businessProfileID.Valid {
			id := businessProfileID.Int64
			inv.BusinessProfileID = &id
		}

		if clientID.Valid {
			id := clientID.Int64
			inv.ClientID = &id
		}

		if dueDate.Valid {
			t := dueDate.Time
			inv.DueDate = &t
		}

		invoices = append(invoices, inv)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("invoice rows iteration error: %w", err)
	}

	return invoices, nil
}

// =====================================================================
// Invoice number uniqueness check
// =====================================================================

// InvoiceNumberExists returns whether an invoice with the given number
// already exists in the database.
func (r *InvoiceRepo) InvoiceNumberExists(ctx context.Context, number string) (bool, error) {

	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM invoices
			WHERE invoice_number = $1
		)
	`

	var exists bool

	err := r.db.QueryRowContext(ctx, query, number).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("invoice number lookup failed: %w", err)
	}

	return exists, nil
}

// ── CHANGE 3: UpdateInvoice — new method ─────────────────────────────
//
// Replaces an existing invoice's data and all its line items inside
// a single transaction. Same authoritative total calculation pattern
// as CreateInvoice(). Items are fully replaced (delete + re-insert)
// which is simpler and safer than diffing individual rows.
func (r *InvoiceRepo) UpdateInvoice(inv *Invoice, items []InvoiceItem) error {

	if inv == nil {
		return errors.New("invoice is required")
	}
	if inv.ID == 0 {
		return errors.New("invoice ID is required for update")
	}
	if inv.InvoiceNumber == "" {
		return errors.New("invoice number is required")
	}

	// Authoritative total recalculation.
	// Never trust totals coming from the form.
	var subtotal float64
	for i := range items {
		line := roundTwo(items[i].Quantity * items[i].UnitPrice)
		items[i].LineTotal = line
		subtotal += line
	}
	subtotal = roundTwo(subtotal)
	taxAmount := roundTwo(subtotal * (inv.TaxRate / 100.0))
	total := roundTwo(subtotal + taxAmount)

	inv.Subtotal = subtotal
	inv.TaxAmount = taxAmount
	inv.Total = total

	if inv.Currency == "" {
		inv.Currency = "USD"
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	const updateInvoice = `
		UPDATE invoices SET
			client_id        = $1,
			client_name      = $2,
			client_email     = $3,
			client_address   = $4,
			client_city      = $5,
			client_zip       = $6,
			client_state     = $7,
			client_country   = $8,
			company_name     = $9,
			company_email    = $10,
			company_address  = $11,
			company_city     = $12,
			company_zip      = $13,
			company_state    = $14,
			company_country  = $15,
			invoice_number   = $16,
			issue_date       = $17,
			due_date         = $18,
			tax_rate         = $19,
			notes            = $20,
			payment_details  = $21,
			subtotal         = $22,
			tax_amount       = $23,
			total            = $24,
			currency         = $25,
			status           = $26,
			updated_at       = NOW()
		WHERE id = $27
	`

	result, err := tx.Exec(
		updateInvoice,
		inv.ClientID,       // $1
		inv.ClientName,     // $2
		inv.ClientEmail,    // $3
		inv.ClientAddress,  // $4
		inv.ClientCity,     // $5
		inv.ClientZip,      // $6
		inv.ClientState,    // $7
		inv.ClientCountry,  // $8
		inv.CompanyName,    // $9
		inv.CompanyEmail,   // $10
		inv.CompanyAddress, // $11
		inv.CompanyCity,    // $12
		inv.CompanyZip,     // $13
		inv.CompanyState,   // $14
		inv.CompanyCountry, // $15
		inv.InvoiceNumber,  // $16
		inv.IssueDate,      // $17
		inv.DueDate,        // $18
		inv.TaxRate,        // $19
		inv.Notes,          // $20
		inv.PaymentDetails, // $21
		inv.Subtotal,       // $22
		inv.TaxAmount,      // $23
		inv.Total,          // $24
		inv.Currency,       // $25
		inv.Status,         // $26
		inv.ID,             // $27
	)
	if err != nil {
		return fmt.Errorf("failed to update invoice: %w", err)
	}

	// Guard: invoice must exist
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to confirm update: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("invoice %d not found", inv.ID)
	}

	// Full replace strategy for line items.
	// DELETE existing → INSERT fresh.
	// ON DELETE CASCADE makes this safe and clean.
	const deleteItems = `
		DELETE FROM invoice_items WHERE invoice_id = $1
	`
	if _, err := tx.Exec(deleteItems, inv.ID); err != nil {
		return fmt.Errorf("failed to delete old invoice items: %w", err)
	}

	const insertItem = `
		INSERT INTO invoice_items (
			invoice_id,
			description,
			quantity,
			unit_price,
			line_total
		) VALUES ($1, $2, $3, $4, $5)
	`
	for _, item := range items {
		if _, err := tx.Exec(
			insertItem,
			inv.ID,
			item.Description,
			item.Quantity,
			item.UnitPrice,
			item.LineTotal,
		); err != nil {
			return fmt.Errorf("failed to insert updated item: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit invoice update: %w", err)
	}

	return nil
}

// ── END CHANGE 3 ──────────────────────────────────────────────────────

// =====================================================================
// Stub methods (kept compatible with previous code)
// =====================================================================

// ListByBusiness returns clients for a given business.
func (r *ClientRepo) ListByBusiness(bizID int64) ([]Client, error) {
	return nil, nil
}

// GetDefault returns the default business profile.
func (r *BusinessRepo) GetDefault() (*BusinessProfile, error) {
	return nil, nil
}
