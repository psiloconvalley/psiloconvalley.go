package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// =====================================================================
// Domain Models
// =====================================================================

type BusinessProfile struct {
	ID                int64
	Name              string
	Email             string
	Address           string
	TaxID             string
	DefaultTaxRateBps int64
	Currency          string
	LogoURL           string
	CreatedAt         time.Time
}

type Client struct {
	ID                int64
	BusinessProfileID int64
	Name              string
	Email             string
	Address           string
	DefaultTaxRateBps int64
	PaymentTerms      string
	CreatedAt         time.Time
}

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	Plan         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (u *User) CheckPassword(plain string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(plain))
	return err == nil
}

type Invoice struct {
	ID                int64
	BusinessProfileID *int64
	ClientID          *int64
	UserID            *int64

	CompanyName    string
	CompanyEmail   string
	CompanyAddress string
	CompanyCity    string
	CompanyZip     string
	CompanyState   string
	CompanyCountry string

	ClientName    string
	ClientEmail   string
	ClientAddress string
	ClientCity    string
	ClientZip     string
	ClientState   string
	ClientCountry string

	InvoiceNumber       string
	IssueDate           time.Time
	DueDate             *time.Time
	TaxRateBps          int64
	DiscountAmountCents int64
	Notes               string
	PaymentDetails      string
	SubtotalCents       int64
	TaxAmountCents      int64
	TotalCents          int64
	Currency            string
	Status              string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type InvoiceItem struct {
	ID             int64
	InvoiceID      int64
	Description    string
	Quantity       float64
	UnitPriceCents int64
	LineTotalCents int64
}

// =====================================================================
// Repositories
// =====================================================================

type InvoiceRepo struct{ db *sql.DB }

func NewInvoiceRepo(db *sql.DB) *InvoiceRepo { return &InvoiceRepo{db: db} }

type ClientRepo struct{ db *sql.DB }

func NewClientRepo(db *sql.DB) *ClientRepo { return &ClientRepo{db: db} }

type BusinessRepo struct{ db *sql.DB }

func NewBusinessRepo(db *sql.DB) *BusinessRepo { return &BusinessRepo{db: db} }

type UserRepo struct{ db *sql.DB }

func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{db: db} }

// =====================================================================
// UserRepo Methods
// =====================================================================

func (r *UserRepo) Create(email, plainPassword string) (int64, error) {
	if email == "" || len(plainPassword) < 8 {
		return 0, errors.New("invalid credentials")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), 12)
	if err != nil {
		return 0, err
	}
	var id int64
	err = r.db.QueryRow(
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
		email, string(hash),
	).Scan(&id)
	return id, err
}

func (r *UserRepo) GetByEmail(email string) (*User, error) {
	var u User
	var updatedAt sql.NullTime
	err := r.db.QueryRow(`
		SELECT id, email, password_hash, plan, created_at, updated_at
		FROM users WHERE email = $1`, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Plan, &u.CreatedAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if updatedAt.Valid {
		u.UpdatedAt = updatedAt.Time
	}
	return &u, nil
}

func (r *UserRepo) GetByID(id int64) (*User, error) {
	var u User
	var updatedAt sql.NullTime
	err := r.db.QueryRow(`
		SELECT id, email, password_hash, plan, created_at, updated_at
		FROM users WHERE id = $1`, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Plan, &u.CreatedAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if updatedAt.Valid {
		u.UpdatedAt = updatedAt.Time
	}
	return &u, nil
}

func (r *UserRepo) GetInvoiceCount(ctx context.Context, userID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM invoices WHERE user_id = $1`, userID,
	).Scan(&count)
	return count, err
}

func (r *UserRepo) IncrementInvoiceCount(userID int64) error {
	// No-op: count is now derived from invoices table via GetInvoiceCount
	return nil
}

// =====================================================================
// Invoice Math
// =====================================================================

func calculateTotals(inv *Invoice, items []InvoiceItem) []InvoiceItem {
	var subtotalCents int64
	for i := range items {
		lineCents := int64(math.Round(items[i].Quantity * float64(items[i].UnitPriceCents)))
		items[i].LineTotalCents = lineCents
		subtotalCents += lineCents
	}
	taxCents := (subtotalCents * inv.TaxRateBps) / 10000
	totalCents := subtotalCents + taxCents - inv.DiscountAmountCents
	if totalCents < 0 {
		totalCents = 0
	}
	inv.SubtotalCents = subtotalCents
	inv.TaxAmountCents = taxCents
	inv.TotalCents = totalCents
	return items
}

// =====================================================================
// InvoiceRepo Methods
// =====================================================================

func (r *InvoiceRepo) CreateInvoice(ctx context.Context, inv *Invoice, items []InvoiceItem) (int64, error) {
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
			business_profile_id, client_id, user_id,
			client_name, client_email, client_address, client_city, client_zip, client_state, client_country,
			company_name, company_email, company_address, company_city, company_zip, company_state, company_country,
			invoice_number, issue_date, due_date,
			tax_rate_bps, discount_amount_cents, notes, payment_details,
			subtotal_cents, tax_amount_cents, total_cents,
			currency, status
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,
			$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,
			$21,$22,$23,$24,$25,$26,$27,$28,$29
		) RETURNING id, created_at`

	var newID int64
	var createdAt time.Time
	err = tx.QueryRowContext(ctx, insertInvoice,
		inv.BusinessProfileID, inv.ClientID, inv.UserID,
		inv.ClientName, inv.ClientEmail, inv.ClientAddress, inv.ClientCity, inv.ClientZip, inv.ClientState, inv.ClientCountry,
		inv.CompanyName, inv.CompanyEmail, inv.CompanyAddress, inv.CompanyCity, inv.CompanyZip, inv.CompanyState, inv.CompanyCountry,
		inv.InvoiceNumber, inv.IssueDate, inv.DueDate,
		inv.TaxRateBps, inv.DiscountAmountCents, inv.Notes, inv.PaymentDetails,
		inv.SubtotalCents, inv.TaxAmountCents, inv.TotalCents,
		inv.Currency, inv.Status,
	).Scan(&newID, &createdAt)
	if err != nil {
		return 0, fmt.Errorf("insert invoice: %w", err)
	}

	inv.ID = newID
	inv.CreatedAt = createdAt

	const insertItem = `
		INSERT INTO invoice_items (invoice_id, description, quantity, unit_price_cents, line_total_cents)
		VALUES ($1, $2, $3, $4, $5)`

	for _, item := range items {
		if _, err := tx.ExecContext(ctx, insertItem,
			newID, item.Description, item.Quantity, item.UnitPriceCents, item.LineTotalCents,
		); err != nil {
			return 0, fmt.Errorf("insert item: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return newID, nil
}

func (r *InvoiceRepo) GetInvoiceWithItems(ctx context.Context, id int64) (*Invoice, []InvoiceItem, error) {
	const q = `
		SELECT 
			id, business_profile_id, client_id, user_id,
			client_name, client_email, client_address, client_city, client_zip, client_state, client_country,
			company_name, company_email, company_address, company_city, company_zip, company_state, company_country,
			invoice_number, issue_date, due_date,
			tax_rate_bps, discount_amount_cents, notes, payment_details,
			subtotal_cents, tax_amount_cents, total_cents,
			currency, status, created_at, updated_at
		FROM invoices WHERE id = $1`

	var inv Invoice
	var bpID, cID, uID sql.NullInt64
	var dueDate, updatedAt sql.NullTime
	var cEmail, cAddr, cCity, cZip, cState, cCountry sql.NullString
	var compName, compEmail, compAddr, compCity, compZip, compState, compCountry sql.NullString
	var currency sql.NullString

	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&inv.ID, &bpID, &cID, &uID,
		&inv.ClientName, &cEmail, &cAddr, &cCity, &cZip, &cState, &cCountry,
		&compName, &compEmail, &compAddr, &compCity, &compZip, &compState, &compCountry,
		&inv.InvoiceNumber, &inv.IssueDate, &dueDate,
		&inv.TaxRateBps, &inv.DiscountAmountCents, &inv.Notes, &inv.PaymentDetails,
		&inv.SubtotalCents, &inv.TaxAmountCents, &inv.TotalCents,
		&currency, &inv.Status, &inv.CreatedAt, &updatedAt,
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
	if compName.Valid {
		inv.CompanyName = compName.String
	}
	if compEmail.Valid {
		inv.CompanyEmail = compEmail.String
	}
	if compAddr.Valid {
		inv.CompanyAddress = compAddr.String
	}
	if compCity.Valid {
		inv.CompanyCity = compCity.String
	}
	if compZip.Valid {
		inv.CompanyZip = compZip.String
	}
	if compState.Valid {
		inv.CompanyState = compState.String
	}
	if compCountry.Valid {
		inv.CompanyCountry = compCountry.String
	}
	if currency.Valid {
		inv.Currency = currency.String
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, invoice_id, description, quantity, unit_price_cents, line_total_cents
		 FROM invoice_items WHERE invoice_id = $1 ORDER BY id`, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var items []InvoiceItem
	for rows.Next() {
		var it InvoiceItem
		if err := rows.Scan(
			&it.ID, &it.InvoiceID, &it.Description,
			&it.Quantity, &it.UnitPriceCents, &it.LineTotalCents,
		); err != nil {
			return nil, nil, err
		}
		items = append(items, it)
	}
	return &inv, items, rows.Err()
}

func (r *InvoiceRepo) ListInvoices(ctx context.Context, limit int, offset int, userID *int64) ([]Invoice, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	q := `SELECT id, user_id, client_name, invoice_number, issue_date, due_date, tax_rate_bps, subtotal_cents, total_cents, status, created_at FROM invoices `
	args := []any{}

	if userID != nil {
		q += `WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		args = append(args, *userID, limit, offset)
	} else {
		q += `ORDER BY created_at DESC LIMIT $1 OFFSET $2`
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
		if err := rows.Scan(
			&inv.ID, &uID, &inv.ClientName, &inv.InvoiceNumber,
			&inv.IssueDate, &dueDate, &inv.TaxRateBps,
			&inv.SubtotalCents, &inv.TotalCents, &inv.Status, &inv.CreatedAt,
		); err != nil {
			return nil, err
		}
		if uID.Valid {
			inv.UserID = &uID.Int64
		}
		if dueDate.Valid {
			inv.DueDate = &dueDate.Time
		}
		invoices = append(invoices, inv)
	}
	return invoices, rows.Err()
}

func (r *InvoiceRepo) InvoiceNumberExists(ctx context.Context, number string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM invoices WHERE invoice_number = $1)`, number,
	).Scan(&exists)
	return exists, err
}

func (r *InvoiceRepo) UpdateInvoice(ctx context.Context, inv *Invoice, items []InvoiceItem) error {
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
			currency=$26, status=$27, updated_at=NOW()
		WHERE id=$28`

	res, err := tx.ExecContext(ctx, uq,
		inv.ClientID, inv.ClientName, inv.ClientEmail, inv.ClientAddress,
		inv.ClientCity, inv.ClientZip, inv.ClientState, inv.ClientCountry,
		inv.CompanyName, inv.CompanyEmail, inv.CompanyAddress,
		inv.CompanyCity, inv.CompanyZip, inv.CompanyState, inv.CompanyCountry,
		inv.InvoiceNumber, inv.IssueDate, inv.DueDate,
		inv.TaxRateBps, inv.DiscountAmountCents, inv.Notes, inv.PaymentDetails,
		inv.SubtotalCents, inv.TaxAmountCents, inv.TotalCents,
		inv.Currency, inv.Status, inv.ID,
	)
	if err != nil {
		return err
	}

	aff, _ := res.RowsAffected()
	if aff == 0 {
		return fmt.Errorf("invoice %d not found", inv.ID)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM invoice_items WHERE invoice_id = $1`, inv.ID,
	); err != nil {
		return fmt.Errorf("delete items: %w", err)
	}

	for _, item := range items {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO invoice_items (invoice_id, description, quantity, unit_price_cents, line_total_cents)
			 VALUES ($1,$2,$3,$4,$5)`,
			inv.ID, item.Description, item.Quantity, item.UnitPriceCents, item.LineTotalCents,
		); err != nil {
			return fmt.Errorf("insert item: %w", err)
		}
	}

	return tx.Commit()
}

// =====================================================================
// Stubs
// =====================================================================

func (r *ClientRepo) ListByBusiness(bizID int64) ([]Client, error) {
	return nil, errors.New("not implemented")
}

func (r *BusinessRepo) GetDefault() (*BusinessProfile, error) {
	return nil, errors.New("not implemented")
}
