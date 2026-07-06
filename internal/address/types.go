// internal/address/types.go
// Shared types for the address autocomplete system.
// Used by local provider, Google provider, and HTTP handlers.
package address

// Suggestion is returned by the autocomplete endpoint.
// Local results include all fields immediately.
// Google results include only Label and PlaceID — full details
// are fetched separately when the user selects a suggestion.
type Suggestion struct {
	Label        string `json:"label"`
	Address      string `json:"address,omitempty"`
	City         string `json:"city,omitempty"`
	State        string `json:"state,omitempty"`
	Zip          string `json:"zip,omitempty"`
	Country      string `json:"country,omitempty"`
	Source       string `json:"source"`          // "local" or "google"
	PlaceID      string `json:"place_id,omitempty"`
	NeedsDetails bool   `json:"needs_details,omitempty"`
}

// PlaceDetails is returned by the place details endpoint.
// Only called once when a Google suggestion is selected.
// Never called for local suggestions.
type PlaceDetails struct {
	Address string `json:"address"`
	City    string `json:"city"`
	State   string `json:"state"`
	Zip     string `json:"zip"`
	Country string `json:"country"`
}
