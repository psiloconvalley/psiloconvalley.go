package repo

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

type BusinessRepo struct{ db *sql.DB }

func NewBusinessRepo(db *sql.DB) *BusinessRepo { return &BusinessRepo{db: db} }

func (r *BusinessRepo) GetByUserID(ctx context.Context, userID int64) (*BusinessProfile, error) {
	var p BusinessProfile
	var city, state, zip, country, email, phone, taxID, currency, logoURL, slug sql.NullString
	var zelleID, venmoHandle, cashappHandle sql.NullString
	var serviceAreas sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, email, address, city, state, zip, country,
		       phone, tax_id, currency, logo_url, slug,
		       zelle_id, venmo_handle, cashapp_handle, service_areas, created_at
		FROM business_profiles
		WHERE user_id = $1
		LIMIT 1
	`, userID).Scan(
		&p.ID, &p.UserID, &p.Name, &email, &p.Address,
		&city, &state, &zip, &country,
		&phone, &taxID, &currency, &logoURL, &slug,
		&zelleID, &venmoHandle, &cashappHandle, &serviceAreas, &p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if email.Valid         { p.Email         = email.String }
	if city.Valid          { p.City          = city.String }
	if state.Valid         { p.State         = state.String }
	if zip.Valid           { p.Zip           = zip.String }
	if country.Valid       { p.Country       = country.String }
	if phone.Valid         { p.Phone         = phone.String }
	if taxID.Valid         { p.TaxID         = taxID.String }
	if currency.Valid      { p.Currency      = currency.String }
	if logoURL.Valid       { p.LogoURL       = logoURL.String }
	if slug.Valid          { p.Slug          = slug.String }
	if zelleID.Valid       { p.ZelleID       = zelleID.String }
	if venmoHandle.Valid   { p.VenmoHandle   = venmoHandle.String }
	if cashappHandle.Valid { p.CashAppHandle = cashappHandle.String }
	if serviceAreas.Valid  { p.ServiceAreas  = serviceAreas.String }
	return &p, nil
}

// GetByID fetches a business profile by its primary key.
// Used when another record stores business_profile_id directly.
func (r *BusinessRepo) GetByID(ctx context.Context, id int64) (*BusinessProfile, error) {
	var p BusinessProfile
	var city, state, zip, country, email, phone, taxID, currency, logoURL, slug sql.NullString
	var zelleID, venmoHandle, cashappHandle sql.NullString
	var serviceAreas sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, email, address, city, state, zip, country,
		       phone, tax_id, currency, logo_url, slug,
		       zelle_id, venmo_handle, cashapp_handle, service_areas, created_at
		FROM business_profiles
		WHERE id = $1
		LIMIT 1
	`, id).Scan(
		&p.ID, &p.UserID, &p.Name, &email, &p.Address,
		&city, &state, &zip, &country,
		&phone, &taxID, &currency, &logoURL, &slug,
		&zelleID, &venmoHandle, &cashappHandle, &serviceAreas, &p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if email.Valid         { p.Email         = email.String }
	if city.Valid          { p.City          = city.String }
	if state.Valid         { p.State         = state.String }
	if zip.Valid           { p.Zip           = zip.String }
	if country.Valid       { p.Country       = country.String }
	if phone.Valid         { p.Phone         = phone.String }
	if taxID.Valid         { p.TaxID         = taxID.String }
	if currency.Valid      { p.Currency      = currency.String }
	if logoURL.Valid       { p.LogoURL       = logoURL.String }
	if slug.Valid          { p.Slug          = slug.String }
	if zelleID.Valid       { p.ZelleID       = zelleID.String }
	if venmoHandle.Valid   { p.VenmoHandle   = venmoHandle.String }
	if cashappHandle.Valid { p.CashAppHandle = cashappHandle.String }
	if serviceAreas.Valid  { p.ServiceAreas  = serviceAreas.String }
	return &p, nil
}

// GetBySlug fetches a business profile by its public slug.
// Used for the public profile page — no auth required.
func (r *BusinessRepo) GetBySlug(ctx context.Context, slug string) (*BusinessProfile, error) {
	var p BusinessProfile
	var city, state, zip, country, email, phone, taxID, currency, logoURL sql.NullString
	var zelleID, venmoHandle, cashappHandle sql.NullString
	var serviceAreas sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, email, address, city, state, zip, country,
		       phone, tax_id, currency, logo_url, slug,
		       zelle_id, venmo_handle, cashapp_handle, service_areas, created_at
		FROM business_profiles
		WHERE slug = $1
		LIMIT 1
	`, slug).Scan(
		&p.ID, &p.UserID, &p.Name, &email, &p.Address,
		&city, &state, &zip, &country,
		&phone, &taxID, &currency, &logoURL, &p.Slug,
		&zelleID, &venmoHandle, &cashappHandle, &serviceAreas, &p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if email.Valid         { p.Email         = email.String }
	if city.Valid          { p.City          = city.String }
	if state.Valid         { p.State         = state.String }
	if zip.Valid           { p.Zip           = zip.String }
	if country.Valid       { p.Country       = country.String }
	if phone.Valid         { p.Phone         = phone.String }
	if taxID.Valid         { p.TaxID         = taxID.String }
	if currency.Valid      { p.Currency      = currency.String }
	if logoURL.Valid       { p.LogoURL       = logoURL.String }
	if zelleID.Valid       { p.ZelleID       = zelleID.String }
	if venmoHandle.Valid   { p.VenmoHandle   = venmoHandle.String }
	if cashappHandle.Valid { p.CashAppHandle = cashappHandle.String }
	if serviceAreas.Valid  { p.ServiceAreas  = serviceAreas.String }
	return &p, nil
}

func (r *BusinessRepo) Upsert(ctx context.Context, p *BusinessProfile) error {
	// Auto-generate slug if empty
	if p.Slug == "" && p.Name != "" {
		slug, err := r.GenerateSlug(ctx, p.Name)
		if err != nil {
			return fmt.Errorf("slug generation: %w", err)
		}
		p.Slug = slug
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO business_profiles
			(user_id, name, email, address, city, state, zip,
			 country, phone, tax_id, currency, logo_url, slug,
			 zelle_id, venmo_handle, cashapp_handle, service_areas)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
			 $14, $15, $16, $17)
		ON CONFLICT (user_id) DO UPDATE SET
			name           = EXCLUDED.name,
			email          = EXCLUDED.email,
			address        = EXCLUDED.address,
			city           = EXCLUDED.city,
			state          = EXCLUDED.state,
			zip            = EXCLUDED.zip,
			country        = EXCLUDED.country,
			phone          = EXCLUDED.phone,
			tax_id         = EXCLUDED.tax_id,
			currency       = EXCLUDED.currency,
			logo_url       = EXCLUDED.logo_url,
			slug           = CASE
				WHEN business_profiles.slug = '' THEN EXCLUDED.slug
				ELSE business_profiles.slug
			END,
			zelle_id       = EXCLUDED.zelle_id,
			venmo_handle   = EXCLUDED.venmo_handle,
			cashapp_handle = EXCLUDED.cashapp_handle,
			service_areas  = EXCLUDED.service_areas
	`,
		p.UserID, p.Name, p.Email, p.Address,
		p.City, p.State, p.Zip, p.Country,
		p.Phone, p.TaxID, p.Currency, p.LogoURL, p.Slug,
		p.ZelleID, p.VenmoHandle, p.CashAppHandle, p.ServiceAreas,
	)
	return err
}

// GenerateSlug creates a URL-safe slug from a business name.
// If the slug already exists, appends -2, -3, etc.
func (r *BusinessRepo) GenerateSlug(ctx context.Context, name string) (string, error) {
	base := slugify(name)
	if base == "" {
		return "", fmt.Errorf("cannot generate slug from empty name")
	}

	candidate := base
	for i := 2; i <= 100; i++ {
		var exists bool
		err := r.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM business_profiles WHERE slug = $1)`,
			candidate,
		).Scan(&exists)
		if err != nil {
			return "", fmt.Errorf("slug check: %w", err)
		}
		if !exists {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
	return "", fmt.Errorf("could not generate unique slug for %q", name)
}

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}
// ClearLogo sets logo_url to NULL for the given user.
// Called when a user removes their logo from the profile page.
func (r *BusinessRepo) ClearLogo(ctx context.Context, userID int64) error {
    _, err := r.db.ExecContext(ctx,
        `UPDATE business_profiles SET logo_url = NULL WHERE user_id = $1`,
        userID,
    )
    return err
}
