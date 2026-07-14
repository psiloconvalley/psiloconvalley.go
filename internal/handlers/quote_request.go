// internal/handlers/quote_request.go
// Authenticated handlers for quote request management.
// Quote requests are created publicly via /biz/{slug}/quote
// and managed here by the business owner.
package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"psiloconvalley/internal/auth"
)

// QuoteRequestMarkViewed marks a quote request as viewed.
// Verifies ownership before updating — fail closed.
// POST /quote-requests/{id}/view
func (h *Handlers) QuoteRequestMarkViewed(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Ownership check — list user's requests and verify this ID belongs to them.
	// We reuse ListByUserID which already JOINs through business_profiles.
	// This is fail-closed: if the ID is not in the user's list, nothing happens.
	requests, err := h.App.QuoteRequestRepo.ListByUserID(r.Context(), user.ID)
	if err != nil {
		slog.Error("quote request ownership check failed", "user_id", user.ID, "err", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	owned := false
	for _, qr := range requests {
		if qr.ID == id {
			owned = true
			break
		}
	}

	if !owned {
		slog.Warn("quote request mark viewed — not owned",
			"user_id", user.ID, "quote_id", id)
		http.NotFound(w, r)
		return
	}

	if err := h.App.QuoteRequestRepo.MarkViewed(r.Context(), id); err != nil {
		slog.Error("quote request mark viewed failed",
			"user_id", user.ID, "quote_id", id, "err", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}
