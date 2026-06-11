package repo

import (
	"context"
	"database/sql"
)

type BusinessRepo struct{ db *sql.DB }

func NewBusinessRepo(db *sql.DB) *BusinessRepo { return &BusinessRepo{db: db} }

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
	if email.Valid { p.Email = email.String }
	if city.Valid { p.City = city.String }
	if state.Valid { p.State = state.String }
	if zip.Valid { p.Zip = zip.String }
	if country.Valid { p.Country = country.String }
	if taxID.Valid { p.TaxID = taxID.String }
	if currency.Valid { p.Currency = currency.String }
	if logoURL.Valid { p.LogoURL = logoURL.String }
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
