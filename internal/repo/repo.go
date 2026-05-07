package repo

import (
	"database/sql"
	"errors"
	"math"
	"time"
)

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

type Client struct {
	ID        int64
	Name      string
	Email     string
	Address   string
	CreatedAt time.Time
}

type Invoice struct {
	ID                int64
	BusinessProfileID *int64
	ClientID          *int64
	ClientName        string
	InvoiceNumber     string
	IssueDate         time.Time
	DueDate           *time.Time
	TaxRate           float64
	Notes             string
	PaymentDetails    string
	Subtotal          float64
	TaxAmount         float64
	Total             float64
	Status            string
	CreatedAt         time.Time
}

type InvoiceItem struct {
	ID          int64
	InvoiceID   int64
	Description string
	Quantity    float64
	UnitPrice   float64
	LineTotal   float64
}

type InvoiceRepo struct {
	db *sql.DB
}

type ClientRepo struct {
	db *sql.DB
}

type BusinessRepo struct {
	db *sql.DB
}

func NewInvoiceRepo(db *sql.DB) *InvoiceRepo {
	return &InvoiceRepo{db: db}
}

func NewClientRepo(db *sql.DB) *ClientRepo {
	return &ClientRepo{db: db}
}

func NewBusinessRepo(db *sql.DB) *BusinessRepo {
	return &BusinessRepo{db: db}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func int64PtrValue(v *int64) any {
	if v == nil {
		return nil
	}

	return *v
}

func timePtrValue(v *time.Time) any {
	if v == nil {
		return nil
	}

	return *v
}

func nullInt64ToPtr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}

	return &v.Int64
}

func nullTimeToPtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}

	return &v.Time
}

func nullStringValue(v sql.NullString) string {
	if !v.Valid {
		return ""
	}

	return v.String
}

// CreateInvoice inserts an invoice and its line items in one database transaction.
func (r *InvoiceRepo) CreateInvoice(inv *Invoice, items []InvoiceItem) (int64, error) {
	if inv == nil {
		return 0, errors.New("invoice is nil")
	}

	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}

	defer tx.Rollback()

	if inv.InvoiceNumber == "" {
		inv.InvoiceNumber = "INV-" + time.Now().Format("20060102150405")
	}

	if inv.Status == "" {
		inv.Status = "draft"
	}

	if inv.IssueDate.IsZero() {
		inv.IssueDate = time.Now()
	}

	subtotal := 0.0

	for _, item := range items {
		lineTotal := item.LineTotal

		if lineTotal == 0 {
			lineTotal = round2(item.Quantity * item.UnitPrice)
		}

		subtotal += lineTotal
	}

	subtotal = round2(subtotal)
	taxAmount := round2(subtotal * inv.TaxRate / 100)
	total := round2(subtotal + taxAmount)

	inv.Subtotal = subtotal
	inv.TaxAmount = taxAmount
	inv.Total = total

	var invoiceID int64
	var createdAt time.Time

	err = tx.QueryRow(`
		INSERT INTO invoices
			(
				business_profile_id,
				client_id,
				invoice_number,
				client_name,
				issue_date,
				due_date,
				tax_rate,
				notes,
				payment_details,
				subtotal,
				tax_amount,
				total,
				status
			)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at
	`,
		int64PtrValue(inv.BusinessProfileID),
		int64PtrValue(inv.ClientID),
		inv.InvoiceNumber,
		inv.ClientName,
		inv.IssueDate,
		timePtrValue(inv.DueDate),
		inv.TaxRate,
		inv.Notes,
		inv.PaymentDetails,
		inv.Subtotal,
		inv.TaxAmount,
		inv.Total,
		inv.Status,
	).Scan(&invoiceID, &createdAt)

	if err != nil {
		return 0, err
	}

	inv.ID = invoiceID
	inv.CreatedAt = createdAt

	for _, item := range items {
		lineTotal := item.LineTotal

		if lineTotal == 0 {
			lineTotal = round2(item.Quantity * item.UnitPrice)
		}

		_, err := tx.Exec(`
			INSERT INTO invoice_items
				(
					invoice_id,
					description,
					quantity,
					unit_price,
					line_total
				)
			VALUES
				($1, $2, $3, $4, $5)
		`,
			invoiceID,
			item.Description,
			item.Quantity,
			item.UnitPrice,
			lineTotal,
		)

		if err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return invoiceID, nil
}

// GetInvoiceWithItems fetches one invoice and all its line items.
func (r *InvoiceRepo) GetInvoiceWithItems(id int64) (*Invoice, []InvoiceItem, error) {
	inv := &Invoice{}

	var businessProfileID sql.NullInt64
	var clientID sql.NullInt64
	var dueDate sql.NullTime
	var notes sql.NullString
	var paymentDetails sql.NullString

	err := r.db.QueryRow(`
		SELECT
			id,
			business_profile_id,
			client_id,
			invoice_number,
			client_name,
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
		WHERE id = $1
	`, id).Scan(
		&inv.ID,
		&businessProfileID,
		&clientID,
		&inv.InvoiceNumber,
		&inv.ClientName,
		&inv.IssueDate,
		&dueDate,
		&inv.TaxRate,
		&notes,
		&paymentDetails,
		&inv.Subtotal,
		&inv.TaxAmount,
		&inv.Total,
		&inv.Status,
		&inv.CreatedAt,
	)

	if err != nil {
		return nil, nil, err
	}

	inv.BusinessProfileID = nullInt64ToPtr(businessProfileID)
	inv.ClientID = nullInt64ToPtr(clientID)
	inv.DueDate = nullTimeToPtr(dueDate)
	inv.Notes = nullStringValue(notes)
	inv.PaymentDetails = nullStringValue(paymentDetails)

	rows, err := r.db.Query(`
		SELECT
			id,
			invoice_id,
			description,
			quantity,
			unit_price,
			line_total
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
			return nil, nil, err
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return inv, items, nil
}

// ListInvoices returns the newest invoices first.
func (r *InvoiceRepo) ListInvoices(limit, offset int) ([]Invoice, error) {
	rows, err := r.db.Query(`
		SELECT
			id,
			business_profile_id,
			client_id,
			invoice_number,
			client_name,
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
	`, limit, offset)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var invoices []Invoice

	for rows.Next() {
		var inv Invoice

		var businessProfileID sql.NullInt64
		var clientID sql.NullInt64
		var dueDate sql.NullTime
		var notes sql.NullString
		var paymentDetails sql.NullString

		err := rows.Scan(
			&inv.ID,
			&businessProfileID,
			&clientID,
			&inv.InvoiceNumber,
			&inv.ClientName,
			&inv.IssueDate,
			&dueDate,
			&inv.TaxRate,
			&notes,
			&paymentDetails,
			&inv.Subtotal,
			&inv.TaxAmount,
			&inv.Total,
			&inv.Status,
			&inv.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		inv.BusinessProfileID = nullInt64ToPtr(businessProfileID)
		inv.ClientID = nullInt64ToPtr(clientID)
		inv.DueDate = nullTimeToPtr(dueDate)
		inv.Notes = nullStringValue(notes)
		inv.PaymentDetails = nullStringValue(paymentDetails)

		invoices = append(invoices, inv)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return invoices, nil
}

// Stub methods. We can properly build these later.
func (r *ClientRepo) ListByBusiness(bizID int64) ([]Client, error) {
	return nil, nil
}

func (r *BusinessRepo) GetDefault() (*BusinessProfile, error) {
	return nil, nil
}
