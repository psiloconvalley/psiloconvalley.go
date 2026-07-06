// internal/address/local.go
// LocalProvider searches the user's own saved client addresses.
// Zero API cost. Always runs first. Results include all fields.
package address

import (
	"context"
	"database/sql"
	"fmt"
)

// LocalProvider queries the user's own client address history.
type LocalProvider struct {
	db *sql.DB
}

// NewLocalProvider creates a LocalProvider backed by the given DB.
func NewLocalProvider(db *sql.DB) *LocalProvider {
	return &LocalProvider{db: db}
}

// Search returns matching addresses from the user's clients.
// Returns full address details — no second API call needed.
func (p *LocalProvider) Search(
	ctx context.Context,
	userID int64,
	query string,
) ([]Suggestion, error) {
	if len(query) < 2 {
		return nil, nil
	}

	rows, err := p.db.QueryContext(ctx, `
		SELECT DISTINCT c.address, c.city, c.state, c.zip, c.country
		FROM clients c
		INNER JOIN business_profiles bp ON bp.id = c.business_profile_id
		WHERE bp.user_id = $1
		AND c.address != ''
		AND (
			c.address ILIKE '%' || $2 || '%'
			OR c.city   ILIKE '%' || $2 || '%'
			OR c.zip    ILIKE '%' || $2 || '%'
		)
		ORDER BY c.address
		LIMIT 5
	`, userID, query)
	if err != nil {
		return nil, fmt.Errorf("local address search: %w", err)
	}
	defer rows.Close()

	var results []Suggestion
	for rows.Next() {
		var s Suggestion
		var city, state, zip, country sql.NullString
		if err := rows.Scan(&s.Address, &city, &state, &zip, &country); err != nil {
			return nil, fmt.Errorf("local address scan: %w", err)
		}
		if city.Valid    { s.City    = city.String }
		if state.Valid   { s.State   = state.String }
		if zip.Valid     { s.Zip     = zip.String }
		if country.Valid { s.Country = country.String }

		// Build a human-readable label
		s.Label  = buildLabel(s.Address, s.City, s.State, s.Zip)
		s.Source = "local"
		// NeedsDetails = false — all fields already populated
		results = append(results, s)
	}
	return results, rows.Err()
}

// buildLabel creates a human-readable one-line address for display.
func buildLabel(address, city, state, zip string) string {
	label := address
	if city != "" {
		label += ", " + city
	}
	if state != "" {
		label += ", " + state
	}
	if zip != "" {
		label += " " + zip
	}
	return label
}
