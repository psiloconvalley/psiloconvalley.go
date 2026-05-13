package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// =====================================================================
// Domain Models
// =====================================================================

type BusinessProfile struct {
	ID        int64
	UserID    int64
	Name      string
	Email     string
	Address   string
	City      string
	State     string
	Zip       string
	Country   string
	TaxID     string
	Currency  string
	LogoURL   string
	CreatedAt time.Time
}

type Client struct {
	ID                int64
	BusinessProfileID int64
	Name              string
	Email             string
	Address           string
	City              string
	State             string
	Zip               string
	Country           string
	Phone             string
	Notes             string
	DefaultTaxRateBps int64
	PaymentTerms      string
	CreatedAt         time.Time
}

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	Plan         string
	Provider     string
	GoogleID     string
	Name         string
	AvatarURL    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (u *User) CheckPassword(plain string) bool {
	if u.PasswordHash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword(
		[]byte(u.PasswordHash),
		[]byte(plain),
	) == nil
}

func (u *User) IsGoogleUser() bool {
	return u.Provider == "google" || u.GoogleID != ""
}

func (u *User) DisplayName() string {
	if u.Name != "" {
		return u.Name
	}
	parts := strings.SplitN(u.Email, "@", 2)
	return parts[0]
}

type Invoice struct {
	ID                int64
	BusinessProfileID *int64
	ClientID          *int64
	UserID            *int64

	LogoURL string

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

// FIX C5 / H5: Details is now a first-class field.
// The DB migration adds `details TEXT NOT NULL DEFAULT ''` to invoice_items.
// This eliminates the newline-concatenation hack in forms.go and surfaces
// details cleanly in the view layer.
type InvoiceItem struct {
	ID             int64
	InvoiceID      int64
	Description    string
	Details        string  // ← NEW: maps to invoice_items.details
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
// UserRepo Methods — unchanged, all correct
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
		`INSERT INTO users (email, password_hash, provider, plan)
		 VALUES ($1, $2, 'email', 'free')
		 RETURNING id`,
		email, string(hash),
	).Scan(&id)
	return id, err
}

func (r *UserRepo) GetByEmail(email string) (*User, error) {
	var u User
	var passwordHash, provider, googleID, name, avatarURL sql.NullString
	var updatedAt sql.NullTime

	err := r.db.QueryRow(`
		SELECT id, email, password_hash, provider, google_id,
		       name, avatar_url, plan, created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(
		&u.ID, &u.Email, &passwordHash, &provider, &googleID,
		&name, &avatarURL, &u.Plan, &u.CreatedAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if passwordHash.Valid { u.PasswordHash = passwordHash.String }
	if provider.Valid     { u.Provider     = provider.String }
	if googleID.Valid     { u.GoogleID     = googleID.String }
	if name.Valid         { u.Name         = name.String }
	if avatarURL.Valid    { u.AvatarURL    = avatarURL.String }
	if updatedAt.Valid    { u.UpdatedAt    = updatedAt.Time }
	return &u, nil
}

func (r *UserRepo) GetByID(id int64) (*User, error) {
	var u User
	var passwordHash, provider, googleID, name, avatarURL sql.NullString
	var updatedAt sql.NullTime

	err := r.db.QueryRow(`
		SELECT id, email, password_hash, provider, google_id,
		       name, avatar_url, plan, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(
		&u.ID, &u.Email, &passwordHash, &provider, &googleID,
		&name, &avatarURL, &u.Plan, &u.CreatedAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if passwordHash.Valid { u.PasswordHash = passwordHash.String }
	if provider.Valid     { u.Provider     = provider.String }
	if googleID.Valid     { u.GoogleID     = googleID.String }
	if name.Valid         { u.Name         = name.String }
	if avatarURL.Valid    { u.AvatarURL    = avatarURL.String }
	if updatedAt.Valid    { u.UpdatedAt    = updatedAt.Time }
	return &u, nil
}

func (r *UserRepo) GetByGoogleID(googleID string) (*User, error) {
	var u User
	var passwordHash, provider, googleIDVal, name, avatarURL sql.NullString
	var updatedAt sql.NullTime

	err := r.db.QueryRow(`
		SELECT id, email, password_hash, provider, google_id,
		       name, avatar_url, plan, created_at, updated_at
		FROM users WHERE google_id = $1
	`, googleID).Scan(
		&u.ID, &u.Email, &passwordHash, &provider, &googleIDVal,
		&name, &avatarURL, &u.Plan, &u.CreatedAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if passwordHash.Valid { u.PasswordHash = passwordHash.String }
	if provider.Valid     { u.Provider     = provider.String }
	if googleIDVal.Valid  { u.GoogleID     = googleIDVal.String }
	if name.Valid         { u.Name         = name.String }
	if avatarURL.Valid    { u.AvatarURL    = avatarURL.String }
	if updatedAt.Valid    { u.UpdatedAt    = updatedAt.Time }
	return &u, nil
}

func (r *UserRepo) GetInvoiceCount(ctx context.Context, userID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM invoices WHERE user_id = $1`, userID,
	).Scan(&count)
	return count, err
}

func (r *UserRepo) GetMonthlyInvoiceCount(ctx context.Context, userID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM invoices
		WHERE user_id = $1
		AND created_at >= date_trunc('month', now())
	`, userID).Scan(&count)
	return count, err
}

func (r *UserRepo) CreateGoogleUser(email, googleID, name, avatarURL string) (int64, error) {
	var id int64
	err := r.db.QueryRow(`
		INSERT INTO users (email, google_id, provider, name, avatar_url, plan)
		VALUES ($1, $2, 'google', $3, $4, 'free')
		RETURNING id
	`, email, googleID, name, avatarURL).Scan(&id)
	return id, err
}

func (r *UserRepo) LinkGoogleToExisting(userID int64, googleID string) error {
	_, err := r.db.Exec(`
		UPDATE users
		SET google_id  = $1,
		    provider   = 'google',
		    updated_at = NOW()
		WHERE id = $2
		AND (google_id IS NULL OR google_id = $1)
	`, googleID, userID)
	return err
}

func (r *UserRepo) FindOrCreateGoogleUser(
	email, googleID, name, avatarURL string,
) (*User, bool, error) {
	user, err := r.GetByGoogleID(googleID)
	if err == nil {
		_, _ = r.db.Exec(`
			UPDATE users
			SET name = $1, avatar_url = $2, updated_at = NOW()
			WHERE id = $3
		`, name, avatarURL, user.ID)
		user.Name      = name
		user.AvatarURL = avatarURL
		return user, false, nil
	}
	if err != sql.ErrNoRows {
		return nil, false, fmt.Errorf("lookup by google id: %w", err)
	}

	user, err = r.GetByEmail(email)
	if err == nil {
		if linkErr := r.LinkGoogleToExisting(user.ID, googleID); linkErr != nil {
			return nil, false, fmt.Errorf("link google: %w", linkErr)
		}
		_, _ = r.db.Exec(`
			UPDATE users
			SET name = $1, avatar_url = $2, updated_at = NOW()
			WHERE id = $3
		`, name, avatarURL, user.ID)
		user.GoogleID  = googleID
		user.Name      = name
		user.AvatarURL = avatarURL
		user.Provider  = "google"
		return user, false, nil
	}
	if err != sql.ErrNoRows {
		return nil, false, fmt.Errorf("lookup by email: %w", err)
	}

	id, err := r.CreateGoogleUser(email, googleID, name, avatarURL)
	if err != nil {
		return nil, false, fmt.Errorf("create google user: %w", err)
	}
	user, err = r.GetByID(id)
	if err != nil {
		return nil, false, fmt.Errorf("fetch new user: %w", err)
	}
	return user, true, nil
}

// =====================================================================
// ClientRepo Methods — unchanged, all correct
// =====================================================================

func (r *ClientRepo) ListByUserID(ctx context.Context, userID int64) ([]Client, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			c.id, c.business_profile_id, c.name, c.email,
			c.address, c.city, c.state, c.zip, c.country,
			c.phone, c.notes, c.payment_terms, c.created_at
		FROM clients c
		INNER JOIN business_profiles bp ON bp.id = c.business_profile_id
		WHERE bp.user_id = $1
		ORDER BY c.name ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []Client
	for rows.Next() {
		var c Client
		var email, address, city, state, zip, country,
			phone, notes, paymentTerms sql.NullString
		if err := rows.Scan(
			&c.ID, &c.BusinessProfileID, &c.Name, &email,
			&address, &city, &state, &zip, &country,
			&phone, &notes, &paymentTerms, &c.CreatedAt,
		); err != nil {
			return nil, err
		}
		if email.Valid        { c.Email        = email.String }
		if address.Valid      { c.Address      = address.String }
		if city.Valid         { c.City         = city.String }
		if state.Valid        { c.State        = state.String }
		if zip.Valid          { c.Zip          = zip.String }
		if country.Valid      { c.Country      = country.String }
		if phone.Valid        { c.Phone        = phone.String }
		if notes.Valid        { c.Notes        = notes.String }
		if paymentTerms.Valid { c.PaymentTerms = paymentTerms.String }
		clients = append(clients, c)
	}
	return clients, rows.Err()
}

func (r *ClientRepo) GetByID(ctx context.Context, id, userID int64) (*Client, error) {
	var c Client
	var email, address, city, state, zip, country,
		phone, notes, paymentTerms sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT
			c.id, c.business_profile_id, c.name, c.email,
			c.address, c.city, c.state, c.zip, c.country,
			c.phone, c.notes, c.payment_terms, c.created_at
		FROM clients c
		INNER JOIN business_profiles bp ON bp.id = c.business_profile_id
		WHERE c.id = $1 AND bp.user_id = $2
	`, id, userID).Scan(
		&c.ID, &c.BusinessProfileID, &c.Name, &email,
		&address, &city, &state, &zip, &country,
		&phone, &notes, &paymentTerms, &c.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if email.Valid        { c.Email        = email.String }
	if address.Valid      { c.Address      = address.String }
	if city.Valid         { c.City         = city.String }
	if state.Valid        { c.State        = state.String }
	if zip.Valid          { c.Zip          = zip.String }
	if country.Valid      { c.Country      = country.String }
	if phone.Valid        { c.Phone        = phone.String }
	if notes.Valid        { c.Notes        = notes.String }
	if paymentTerms.Valid { c.PaymentTerms = paymentTerms.String }
	return &c, nil
}

func (r *ClientRepo) Create(ctx context.Context, c *Client) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO clients
			(business_profile_id, name, email, address, city,
			 state, zip, country, phone, notes, payment_terms)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`,
		c.BusinessProfileID, c.Name, c.Email, c.Address,
		c.City, c.State, c.Zip, c.Country,
		c.Phone, c.Notes, c.PaymentTerms,
	).Scan(&id)
	return id, err
}

func (r *ClientRepo) Update(ctx context.Context, c *Client, userID int64) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE clients SET
			name          = $1,
			email         = $2,
			address       = $3,
			city          = $4,
			state         = $5,
			zip           = $6,
			country       = $7,
			phone         = $8,
			notes         = $9,
			payment_terms = $10
		WHERE id = $11
		AND business_profile_id IN (
			SELECT id FROM business_profiles WHERE user_id = $12
		)
	`,
		c.Name, c.Email, c.Address,
		c.City, c.State, c.Zip, c.Country,
		c.Phone, c.Notes, c.PaymentTerms,
		c.ID, userID,
	)
	if err != nil {
		return err
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return fmt.Errorf("client %d not found or access denied", c.ID)
	}
	return nil
}

func (r *ClientRepo) Delete(ctx context.Context, id, userID int64) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM clients
		WHERE id = $1
		AND business_profile_id IN (
			SELECT id FROM business_profiles WHERE user_id = $2
		)
	`, id, userID)
	if err != nil {
		return err
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return fmt.Errorf("client %d not found or access denied", id)
	}
	return nil
}

func (r *ClientRepo) CountByUserID(ctx context.Context, userID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM clients c
		INNER JOIN business_profiles bp ON bp.id = c.business_profile_id
		WHERE bp.user_id = $1
	`, userID).Scan(&count)
	return count, err
}

func (r *ClientRepo) FindOrCreate(
	ctx context.Context,
	bizProfileID int64,
	name, email, address, city, state, zip, country string,
) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		SELECT id FROM clients
		WHERE business_profile_id = $1
		AND LOWER(TRIM(name)) = LOWER(TRIM($2))
		LIMIT 1
	`, bizProfileID, name).Scan(&id)

	if err == nil {
		_, _ = r.db.ExecContext(ctx, `
			UPDATE clients SET
				email   = COALESCE(NULLIF($1, ''), email),
				address = COALESCE(NULLIF($2, ''), address),
				city    = COALESCE(NULLIF($3, ''), city),
				state   = COALESCE(NULLIF($4, ''), state),
				zip     = COALESCE(NULLIF($5, ''), zip),
				country = COALESCE(NULLIF($6, ''), country)
			WHERE id = $7
		`, email, address, city, state, zip, country, id)
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("client lookup: %w", err)
	}

	err = r.db.QueryRowContext(ctx, `
		INSERT INTO clients
			(business_profile_id, name, email, address, city, state, zip, country)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, bizProfileID, name, email, address, city, state, zip, country).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("client create: %w", err)
	}
	return id, nil
}

// =====================================================================
// BusinessRepo Methods — unchanged, all correct
// =====================================================================

func (r *BusinessRepo) GetByUserID(ctx context.Context, userID int64) (*BusinessProfile, error) {
	var p BusinessProfile
	var city, state, zip, country, email, taxID, currency, logoURL sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, email, address, city, state, zip, country,
		       tax_id, currency, logo_url, created_at
		FROM business_profiles
		WHERE user_id = $1
		LIMIT 1
	`, userID).Scan(
		&p.ID, &p.UserID, &p.Name, &email, &p.Address,
		&city, &state, &zip, &country,
		&taxID, &currency, &logoURL, &p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if email.Valid    { p.Email    = email.String }
	if city.Valid     { p.City     = city.String }
	if state.Valid    { p.State    = state.String }
	if zip.Valid      { p.Zip      = zip.String }
	if country.Valid  { p.Country  = country.String }
	if taxID.Valid    { p.TaxID    = taxID.String }
	if currency.Valid { p.Currency = currency.String }
	if logoURL.Valid  { p.LogoURL  = logoURL.String }
	return &p, nil
}

func (r *BusinessRepo) Upsert(ctx context.Context, p *BusinessProfile) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO business_profiles
			(user_id, name, email, address, city, state, zip,
			 country, tax_id, currency, logo_url)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (user_id) DO UPDATE SET
			name     = EXCLUDED.name,
			email    = EXCLUDED.email,
			address  = EXCLUDED.address,
			city     = EXCLUDED.city,
			state    = EXCLUDED.state,
			zip      = EXCLUDED.zip,
			country  = EXCLUDED.country,
			tax_id   = EXCLUDED.tax_id,
			currency = EXCLUDED.currency,
			logo_url = EXCLUDED.logo_url
	`,
		p.UserID, p.Name, p.Email, p.Address,
		p.City, p.State, p.Zip, p.Country,
		p.TaxID, p.Currency, p.LogoURL,
	)
	return err
}

func (r *BusinessRepo) GetDefault() (*BusinessProfile, error) {
	return nil, errors.New("not implemented")
}

// =====================================================================
// Invoice Math
// =====================================================================

// calculateTotals computes line totals, subtotal, tax, and grand total.
//
// FIX H1: Discount is now capped at subtotal+tax before being applied.
// Previously a discount exceeding the invoice total would floor to $0
// while storing internally inconsistent values (subtotal=$10, discount=$20,
// total=$0). Now the stored discount never exceeds what it can actually
// reduce, keeping the accounting record self-consistent.
func calculateTotals(inv *Invoice, items []InvoiceItem) []InvoiceItem {
	var subtotalCents int64
	for i := range items {
		lineCents := int64(math.Round(
			items[i].Quantity * float64(items[i].UnitPriceCents),
		))
		items[i].LineTotalCents = lineCents
		subtotalCents += lineCents
	}

	taxCents := (subtotalCents * inv.TaxRateBps) / 10000
	preTaxTotal := subtotalCents + taxCents

	// Cap discount so stored values remain internally consistent.
	// A discount cannot exceed the pre-discount total.
	if inv.DiscountAmountCents > preTaxTotal {
		inv.DiscountAmountCents = preTaxTotal
	}
	if inv.DiscountAmountCents < 0 {
		inv.DiscountAmountCents = 0
	}

	inv.SubtotalCents  = subtotalCents
	inv.TaxAmountCents = taxCents
	inv.TotalCents     = preTaxTotal - inv.DiscountAmountCents
	return items
}

// =====================================================================
// InvoiceRepo Methods
// =====================================================================

func (r *InvoiceRepo) CreateInvoice(
	ctx context.Context,
	inv *Invoice,
	items []InvoiceItem,
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
			business_profile_id, client_id, user_id,
			client_name, client_email, client_address,
			client_city, client_zip, client_state, client_country,
			company_name, company_email, company_address,
			company_city, company_zip, company_state, company_country,
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
		inv.ClientName, inv.ClientEmail, inv.ClientAddress,
		inv.ClientCity, inv.ClientZip, inv.ClientState, inv.ClientCountry,
		inv.CompanyName, inv.CompanyEmail, inv.CompanyAddress,
		inv.CompanyCity, inv.CompanyZip, inv.CompanyState, inv.CompanyCountry,
		inv.InvoiceNumber, inv.IssueDate, inv.DueDate,
		inv.TaxRateBps, inv.DiscountAmountCents, inv.Notes, inv.PaymentDetails,
		inv.SubtotalCents, inv.TaxAmountCents, inv.TotalCents,
		inv.Currency, inv.Status,
	).Scan(&newID, &createdAt)
	if err != nil {
		return 0, fmt.Errorf("insert invoice: %w", err)
	}

	inv.ID        = newID
	inv.CreatedAt = createdAt

	// FIX C5: INSERT now includes the details column.
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

func (r *InvoiceRepo) GetInvoiceWithItems(
	ctx context.Context,
	id int64,
) (*Invoice, []InvoiceItem, error) {
	const q = `
		SELECT
			id, business_profile_id, client_id, user_id,
			client_name, client_email, client_address,
			client_city, client_zip, client_state, client_country,
			company_name, company_email, company_address,
			company_city, company_zip, company_state, company_country,
			invoice_number, issue_date, due_date,
			tax_rate_bps, discount_amount_cents, notes, payment_details,
			subtotal_cents, tax_amount_cents, total_cents,
			currency, status, created_at, updated_at
		FROM invoices WHERE id = $1`

	var inv Invoice
	var bpID, cID, uID sql.NullInt64
	var dueDate, updatedAt sql.NullTime
	var cEmail, cAddr, cCity, cZip, cState, cCountry sql.NullString
	var compName, compEmail, compAddr, compCity, compZip,
		compState, compCountry sql.NullString
	var currency sql.NullString

	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&inv.ID, &bpID, &cID, &uID,
		&inv.ClientName, &cEmail, &cAddr, &cCity, &cZip, &cState, &cCountry,
		&compName, &compEmail, &compAddr, &compCity, &compZip,
		&compState, &compCountry,
		&inv.InvoiceNumber, &inv.IssueDate, &dueDate,
		&inv.TaxRateBps, &inv.DiscountAmountCents, &inv.Notes, &inv.PaymentDetails,
		&inv.SubtotalCents, &inv.TaxAmountCents, &inv.TotalCents,
		&currency, &inv.Status, &inv.CreatedAt, &updatedAt,
	)
	if err != nil {
		return nil, nil, err
	}

	if bpID.Valid       { inv.BusinessProfileID = &bpID.Int64 }
	if cID.Valid        { inv.ClientID          = &cID.Int64 }
	if uID.Valid        { inv.UserID            = &uID.Int64 }
	if dueDate.Valid    { inv.DueDate           = &dueDate.Time }
	if updatedAt.Valid  { inv.UpdatedAt         = updatedAt.Time }
	if cEmail.Valid     { inv.ClientEmail       = cEmail.String }
	if cAddr.Valid      { inv.ClientAddress     = cAddr.String }
	if cCity.Valid      { inv.ClientCity        = cCity.String }
	if cZip.Valid       { inv.ClientZip         = cZip.String }
	if cState.Valid     { inv.ClientState       = cState.String }
	if cCountry.Valid   { inv.ClientCountry     = cCountry.String }
	if compName.Valid   { inv.CompanyName       = compName.String }
	if compEmail.Valid  { inv.CompanyEmail      = compEmail.String }
	if compAddr.Valid   { inv.CompanyAddress    = compAddr.String }
	if compCity.Valid   { inv.CompanyCity       = compCity.String }
	if compZip.Valid    { inv.CompanyZip        = compZip.String }
	if compState.Valid  { inv.CompanyState      = compState.String }
	if compCountry.Valid { inv.CompanyCountry   = compCountry.String }
	if currency.Valid   { inv.Currency          = currency.String }

	// FIX C5: SELECT now includes details column.
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
		if err := rows.Scan(
			&it.ID, &it.InvoiceID, &it.Description, &it.Details,
			&it.Quantity, &it.UnitPriceCents, &it.LineTotalCents,
		); err != nil {
			return nil, nil, err
		}
		items = append(items, it)
	}
	return &inv, items, rows.Err()
}

// FIX C2: currency is now included in the SELECT so MapInvoiceList
// can render the correct symbol for every non-USD invoice.
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
	             subtotal_cents, total_cents, currency, status, created_at
	      FROM invoices `

	var args []any
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
		var currency sql.NullString
		if err := rows.Scan(
			&inv.ID, &uID, &inv.ClientName, &inv.InvoiceNumber,
			&inv.IssueDate, &dueDate, &inv.TaxRateBps,
			&inv.SubtotalCents, &inv.TotalCents, &currency,
			&inv.Status, &inv.CreatedAt,
		); err != nil {
			return nil, err
		}
		if uID.Valid      { inv.UserID   = &uID.Int64 }
		if dueDate.Valid  { inv.DueDate  = &dueDate.Time }
		if currency.Valid { inv.Currency = currency.String }
		invoices = append(invoices, inv)
	}
	return invoices, rows.Err()
}

func (r *InvoiceRepo) InvoiceNumberExists(
	ctx context.Context,
	number string,
) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM invoices WHERE invoice_number = $1)`,
		number,
	).Scan(&exists)
	return exists, err
}

// FIX C1: WHERE clause now includes AND user_id=$29.
// Previously UpdateInvoice relied solely on the handler's canAccessInvoice
// check. Defense in depth requires the DB operation itself to be scoped
// to the owning user — a single misconfigured middleware cannot escalate
// to cross-user data mutation.
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
			currency=$26, status=$27, updated_at=NOW()
		WHERE id=$28 AND user_id=$29`

	res, err := tx.ExecContext(ctx, uq,
		inv.ClientID, inv.ClientName, inv.ClientEmail, inv.ClientAddress,
		inv.ClientCity, inv.ClientZip, inv.ClientState, inv.ClientCountry,
		inv.CompanyName, inv.CompanyEmail, inv.CompanyAddress,
		inv.CompanyCity, inv.CompanyZip, inv.CompanyState, inv.CompanyCountry,
		inv.InvoiceNumber, inv.IssueDate, inv.DueDate,
		inv.TaxRateBps, inv.DiscountAmountCents, inv.Notes, inv.PaymentDetails,
		inv.SubtotalCents, inv.TaxAmountCents, inv.TotalCents,
		inv.Currency, inv.Status,
		inv.ID, inv.UserID, // $28, $29
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

	// FIX C5: INSERT now persists details independently.
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

// UpdateInvoiceStatus — unchanged, was already correct.
// State machine validation and user_id scoping were already in place.
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
