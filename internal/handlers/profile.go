package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/repo"
)

func (h *Handlers) ProfileGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	profile, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("profile fetch error: %v", err)
		http.Error(w, "Could not load profile", http.StatusInternalServerError)
		return
	}

	h.App.Render(w, r, "profile.tmpl", map[string]any{
		"Profile":    profile,
		"Saved":      r.URL.Query().Get("saved") == "true",
		"Welcome":    r.URL.Query().Get("welcome") == "true",
		"Currencies": catalog.SupportedCurrencies,
	})
}

func (h *Handlers) ProfilePost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	p := &repo.BusinessProfile{
		UserID:  user.ID,
		Name:    strings.TrimSpace(r.FormValue("name")),
		Email:   strings.TrimSpace(r.FormValue("email")),
		Address: strings.TrimSpace(r.FormValue("address")),
		City:    strings.TrimSpace(r.FormValue("city")),
		State:   strings.TrimSpace(r.FormValue("state")),
		Zip:     strings.TrimSpace(r.FormValue("zip")),
		Country: strings.TrimSpace(r.FormValue("country")),
		TaxID:   strings.TrimSpace(r.FormValue("tax_id")),
		Currency: func() string {
			c := r.FormValue("currency")
			if c == "" {
				return "USD"
			}
			return c
		}(),
	}

	if p.Name == "" {
		h.App.Render(w, r, "profile.tmpl", map[string]any{
			"Profile": p, "Error": "Company name is required", "Currencies": catalog.SupportedCurrencies,
		})
		return
	}

	if err := h.App.BizRepo.Upsert(r.Context(), p); err != nil {
		log.Printf("profile upsert error: %v", err)
		h.App.Render(w, r, "profile.tmpl", map[string]any{
			"Profile": p, "Error": "Could not save profile", "Currencies": catalog.SupportedCurrencies,
		})
		return
	}

	http.Redirect(w, r, "/profile?saved=true", http.StatusSeeOther)
}
