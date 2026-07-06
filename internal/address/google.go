// internal/address/google.go
// GoogleProvider wraps the Google Places API (New).
// Two operations:
//   Autocomplete — returns suggestions with place_id (cheap)
//   Details      — returns full address for a place_id (called once on selection)
//
// Cost protection:
//   - Only called when local results < localThreshold
//   - Only called when query length >= minQueryLen
//   - Results cached for cacheTTL minutes
//   - Fails open — never blocks the user
package address

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	googleAutocompleteURL = "https://places.googleapis.com/v1/places:autocomplete"
	googleDetailsURL      = "https://places.googleapis.com/v1/places/%s"
	googleTimeout         = 3 * time.Second
	minQueryLen           = 5 // minimum query length before calling Google
	localThreshold        = 3 // call Google only if local returns fewer than this
)

// GoogleProvider calls the Google Places API.
type GoogleProvider struct {
	apiKey string
	client *http.Client
}

// NewGoogleProvider creates a GoogleProvider.
// If apiKey is empty, all methods return empty results (kill switch).
func NewGoogleProvider(apiKey string) *GoogleProvider {
	return &GoogleProvider{
		apiKey: apiKey,
		client: &http.Client{Timeout: googleTimeout},
	}
}

// Enabled returns true when a Google API key is configured.
func (p *GoogleProvider) Enabled() bool {
	return p.apiKey != ""
}

// Autocomplete returns place suggestions for a query string.
// Returns only label + place_id — details fetched separately on selection.
func (p *GoogleProvider) Autocomplete(
	ctx context.Context,
	query string,
) ([]Suggestion, error) {
	if !p.Enabled() {
		return nil, nil
	}

	body, _ := json.Marshal(map[string]any{
		"input":               query,
		"includedRegionCodes": []string{"us", "mx"},
		"languageCode":        "en",
		"includedPrimaryTypes": []string{
			"street_address",
			"premise",
			"subpremise",
		},
	})

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, googleAutocompleteURL, bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("google autocomplete request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", p.apiKey)
	req.Header.Set("X-Goog-FieldMask",
		"suggestions.placePrediction.placeId,suggestions.placePrediction.text")

	resp, err := p.client.Do(req)
	if err != nil {
		slog.Warn("google autocomplete failed", "err", err)
		return nil, nil // fail open
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		slog.Warn("google autocomplete non-200",
			"status", resp.StatusCode, "body", string(b))
		return nil, nil // fail open
	}

	var raw struct {
		Suggestions []struct {
			PlacePrediction struct {
				PlaceID string `json:"placeId"`
				Text    struct {
					Text string `json:"text"`
				} `json:"text"`
			} `json:"placePrediction"`
		} `json:"suggestions"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		slog.Warn("google autocomplete decode failed", "err", err)
		return nil, nil
	}

	var results []Suggestion
	for _, s := range raw.Suggestions {
		pp := s.PlacePrediction
		if pp.PlaceID == "" || pp.Text.Text == "" {
			continue
		}
		results = append(results, Suggestion{
			Label:        pp.Text.Text,
			Source:       "google",
			PlaceID:      pp.PlaceID,
			NeedsDetails: true,
		})
	}
	return results, nil
}

// Details fetches full address components for a Google place_id.
// Called once when the user selects a Google suggestion.
func (p *GoogleProvider) Details(
	ctx context.Context,
	placeID string,
) (*PlaceDetails, error) {
	if !p.Enabled() || placeID == "" {
		return nil, nil
	}

	url := fmt.Sprintf(googleDetailsURL, placeID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("google details request: %w", err)
	}
	req.Header.Set("X-Goog-Api-Key", p.apiKey)
	req.Header.Set("X-Goog-FieldMask", "addressComponents,shortFormattedAddress")

	resp, err := p.client.Do(req)
	if err != nil {
		slog.Warn("google place details failed", "err", err, "place_id", placeID)
		return nil, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		slog.Warn("google place details non-200",
			"status", resp.StatusCode, "body", string(b))
		return nil, nil
	}

	var raw struct {
		ShortFormattedAddress string `json:"shortFormattedAddress"`
		AddressComponents     []struct {
			LongText  string   `json:"longText"`
			ShortText string   `json:"shortText"`
			Types     []string `json:"types"`
		} `json:"addressComponents"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		slog.Warn("google place details decode failed", "err", err)
		return nil, nil
	}

	d := &PlaceDetails{}
	var streetNumber, route string

	for _, c := range raw.AddressComponents {
		for _, t := range c.Types {
			switch t {
			case "street_number":
				streetNumber = c.LongText
			case "route":
				route = c.LongText
			case "locality":
				d.City = c.LongText
			case "administrative_area_level_1":
				d.State = c.LongText
			case "postal_code":
				d.Zip = c.LongText
			case "country":
				d.Country = c.LongText
			}
		}
	}

	if streetNumber != "" && route != "" {
		d.Address = streetNumber + " " + route
	} else if raw.ShortFormattedAddress != "" {
		d.Address = raw.ShortFormattedAddress
	}

	return d, nil
}
