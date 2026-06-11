package repo

import (
	"context"
	"database/sql"
	"fmt"
)

type ClientRepo struct{ db *sql.DB }

func NewClientRepo(db *sql.DB) *ClientRepo { return &ClientRepo{db: db} }

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
		var email, address, city, state, zip, country, phone, notes, paymentTerms sql.NullString
		if err := rows.Scan(
			&c.ID, &c.BusinessProfileID, &c.Name, &email,
			&address, &city, &state, &zip, &country,
			&phone, &notes, &paymentTerms, &c.CreatedAt,
		); err != nil {
			return nil, err
		}
		if email.Valid { c.Email = email.String }
		if address.Valid { c.Address = address.String }
		if city.Valid { c.City = city.String }
		if state.Valid { c.State = state.String }
		if zip.Valid { c.Zip = zip.String }
		if country.Valid { c.Country = country.String }
		if phone.Valid { c.Phone = phone.String }
		if notes.Valid { c.Notes = notes.String }
		if paymentTerms.Valid { c.PaymentTerms = paymentTerms.String }
		clients = append(clients, c)
	}
	return clients, rows.Err()
}

func (r *ClientRepo) GetByID(ctx context.Context, id, userID int64) (*Client, error) {
	var c Client
	var email, address, city, state, zip, country, phone, notes, paymentTerms sql.NullString
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
	if email.Valid { c.Email = email.String }
	if address.Valid { c.Address = address.String }
	if city.Valid { c.City = city.String }
	if state.Valid { c.State = state.String }
	if zip.Valid { c.Zip = zip.String }
	if country.Valid { c.Country = country.String }
	if phone.Valid { c.Phone = phone.String }
	if notes.Valid { c.Notes = notes.String }
	if paymentTerms.Valid { c.PaymentTerms = paymentTerms.String }
	return &c, nil
}

func (r *ClientRepo) Create(ctx context.Context, c *Client) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO clients
			(business_profile_id, name, email, address, city,
			 state, zip, country, phone, notes, payment_terms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
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
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, bizProfileID, name, email, address, city, state, zip, country).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("client create: %w", err)
	}
	return id, nil
}
