// internal/handlers/address.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/repo"
)

// AddressAutocompleteGet returns matching addresses from the user's clients.
// GET /api/addresses?q=...
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

	results, err := h.App.AddressRepo.SearchByUser(r.Context(), user.ID, query)
	if err != nil {
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	if results == nil {
		results = make([]repo.AddressResult, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
