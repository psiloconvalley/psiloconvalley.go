package repo

import (
	"database/sql"
	"time"
)

type Business struct {
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

type Invoice struct {
	ID                int64
	BusinessProfileID int64
	ClientID          *int64
	InvoiceNumber     string
	IssueDate         time.Time
	DueDate           *time.Time
	Status            string
	TaxRate           float64
	DiscountAmount    float64
	Notes             string
	PaymentDetails    string
	PdfURL            string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type InvoiceItem struct {
	ID          int64
	InvoiceID   int64
	Description string
	Quantity    float64
	UnitPrice   float64
	LineTotal   float64
}

type InvoiceRepo struct{ db *sql.DB }
type ClientRepo struct{ db *sql.DB }
type BusinessRepo struct{ db *sql.DB }

func NewInvoiceRepo(db *sql.DB) *InvoiceRepo   { return &InvoiceRepo{db} }
func NewClientRepo(db *sql.DB) *ClientRepo     { return &ClientRepo{db} }
func NewBusinessRepo(db *sql.DB) *BusinessRepo { return &BusinessRepo{db} }

// CreateInvoice inserts invoice + items atomically
func (r *InvoiceRepo) CreateInvoice(inv *Invoice, items []InvoiceItem) (int64, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var invoiceID int64
	err = tx.QueryRow(`
		INSERT INTO invoices
		  (business_profile_id, client_id, invoice_number, issue_date, due_date, status, tax_rate, discount_amount, notes, payment_details)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`,
		inv.BusinessProfileID,
		inv.ClientID,
		inv.InvoiceNumber,
		inv.IssueDate,
		inv.DueDate,
		inv.Status,
		inv.TaxRate,
		inv.DiscountAmount,
		inv.Notes,
		inv.PaymentDetails,
	).Scan(&invoiceID)
	if err != nil {
		return 0, err
	}

	for _, it := range items {
		_, err := tx.Exec(`
			INSERT INTO invoice_items (invoice_id, description, quantity, unit_price)
			VALUES ($1, $2, $3, $4)
		`, invoiceID, it.Description, it.Quantity, it.UnitPrice)
		if err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return invoiceID, nil
}

// GetInvoiceWithItems fetches an invoice and its items
func (r *InvoiceRepo) GetInvoiceWithItems(id int64) (*Invoice, []InvoiceItem, error) {
	inv := &Invoice{}
	err := r.db.QueryRow(`
		SELECT id, business_profile_id, client_id, invoice_number, issue_date, due_date, status,
		       tax_rate, discount_amount, notes, payment_details, pdf_url, created_at, updated_at
		FROM invoices
		WHERE id = $1
	`, id).Scan(
		&inv.ID,
		&inv.BusinessProfileID,
		&inv.ClientID,
		&inv.InvoiceNumber,
		&inv.IssueDate,
		&inv.DueDate,
		&inv.Status,
		&inv.TaxRate,
		&inv.DiscountAmount,
		&inv.Notes,
		&inv.PaymentDetails,
		&inv.PdfURL,
		&inv.CreatedAt,
		&inv.UpdatedAt,
	)
	if err != nil {
		return nil, nil, err
	}

	rows, err := r.db.Query(`
		SELECT id, invoice_id, description, quantity, unit_price,
		       COALESCE(line_total, quantity * unit_price) AS line_total
		FROM invoice_items
		WHERE invoice_id = $1
		ORDER BY id
	`, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var items []InvoiceItem
	for rows.Next() {
		var it InvoiceItem
		if err := rows.Scan(
			&it.ID,
			&it.InvoiceID,
			&it.Description,
			&it.Quantity,
			&it.UnitPrice,
			&it.LineTotal,
		); err != nil {
			return nil, nil, err
		}
		items = append(items, it)
	}

	return inv, items, nil
}

// ListInvoices returns paginated invoices (newest first)
func (r *InvoiceRepo) ListInvoices(limit, offset int) ([]Invoice, error) {
	rows, err := r.db.Query(`
		SELECT id, business_profile_id, client_id, invoice_number, issue_date, due_date, status,
		       tax_rate, discount_amount, notes, payment_details, pdf_url, created_at, updated_at
		FROM invoices
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Invoice
	for rows.Next() {
		var inv Invoice
		if err := rows.Scan(
			&inv.ID,
			&inv.BusinessProfileID,
			&inv.ClientID,
			&inv.InvoiceNumber,
			&inv.IssueDate,
			&inv.DueDate,
			&inv.Status,
			&inv.TaxRate,
			&inv.DiscountAmount,
			&inv.Notes,
			&inv.PaymentDetails,
			&inv.PdfURL,
			&inv.CreatedAt,
			&inv.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, inv)
	}

	return list, nil
}

// Stub methods (implement when you need them)
func (r *ClientRepo) ListByBusiness(bizID int64) ([]Client, error) { return nil, nil }
func (r *BusinessRepo) GetDefault() (*Business, error)             { return nil, nil }
