// internal/handlers/address.go
// HTTP handlers for address autocomplete.
// Two endpoints:
//   GET /api/addresses?q=...     — suggestions (local + Google)
//   GET /api/addresses/place?id= — Google place details (on selection)
package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"psiloconvalley/internal/address"
	"psiloconvalley/internal/auth"
)

// AddressAutocompleteGet returns matching address suggestions.
// Local results come first. Google results fill in if needed.
func (h *Handlers) AddressAutocompleteGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) < 2 {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}

	results, err := h.App.AddressService.Suggest(r.Context(), user.ID, query)
	if err != nil {
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	if results == nil {
		results = make([]address.Suggestion, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// AddressPlaceDetailsGet fetches full address for a Google place_id.
// Called once when the user selects a Google suggestion.
func (h *Handlers) AddressPlaceDetailsGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	placeID := strings.TrimSpace(r.URL.Query().Get("id"))
	if placeID == "" {
		http.Error(w, "Missing place ID", http.StatusBadRequest)
		return
	}

	details, err := h.App.AddressService.PlaceDetails(r.Context(), placeID)
	if err != nil || details == nil {
		http.Error(w, "Place details failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(details)
}
