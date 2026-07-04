// internal/repo/address_repo.go
package repo

import (
	"context"
	"database/sql"
)

// AddressResult represents a distinct address from a user's clients.
type AddressResult struct {
	Address string `json:"address"`
	City    string `json:"city"`
	State   string `json:"state"`
	Zip     string `json:"zip"`
	Country string `json:"country"`
}

type AddressRepo struct {
	db *sql.DB
}

func NewAddressRepo(db *sql.DB) *AddressRepo {
	return &AddressRepo{db: db}
}

// SearchByUser returns distinct addresses from a user's clients
// matching the query string against address, city, or zip.
func (r *AddressRepo) SearchByUser(
	ctx context.Context,
	userID int64,
	query string,
) ([]AddressResult, error) {
	if len(query) < 2 {
		return nil, nil
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT c.address, c.city, c.state, c.zip, c.country
		FROM clients c
		INNER JOIN business_profiles bp ON bp.id = c.business_profile_id
		WHERE bp.user_id = $1
		AND c.address != ''
		AND (
			c.address ILIKE '%' || $2 || '%'
			OR c.city ILIKE '%' || $2 || '%'
			OR c.zip ILIKE '%' || $2 || '%'
		)
		ORDER BY c.address
		LIMIT 5
	`, userID, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []AddressResult
	for rows.Next() {
		var a AddressResult
		var city, state, zip, country sql.NullString
		if err := rows.Scan(&a.Address, &city, &state, &zip, &country); err != nil {
			return nil, err
		}
		if city.Valid {
			a.City = city.String
		}
		if state.Valid {
			a.State = state.String
		}
		if zip.Valid {
			a.Zip = zip.String
		}
		if country.Valid {
			a.Country = country.String
		}
		results = append(results, a)
	}
	return results, rows.Err()
}
