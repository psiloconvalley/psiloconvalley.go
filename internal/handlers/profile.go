package handlers

import (
	"database/sql"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/logo"
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
	   "Profile":         profile,
	   "Saved":           r.URL.Query().Get("saved") == "true",
	   "Welcome":         r.URL.Query().Get("welcome") == "true",
	   "Currencies":      catalog.SupportedCurrencies,
	   "StripeConnected": r.URL.Query().Get("stripe_connected") == "1",
	   "StripeError":     r.URL.Query().Get("stripe_error") == "1",
	})
	}

func (h *Handlers) ProfilePost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Max upload size: 2MB
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		log.Printf("multipart parse error: %v", err)

		h.App.Render(w, r, "profile.tmpl", map[string]any{
			"Error":      "Upload too large or invalid form data",
			"Currencies": catalog.SupportedCurrencies,
		})
		return
	}

	// Load existing profile first so we preserve current logo
	existing, _ := h.App.BizRepo.GetByUserID(r.Context(), user.ID)

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
			c := strings.TrimSpace(r.FormValue("currency"))
			if c == "" {
				return "USD"
			}
			return c
		}(),
	}

	// Preserve existing logo by default
	if existing != nil {
		p.LogoURL = existing.LogoURL
	}

	// ============================================================
	// Logo Upload Handling — Top Class
	// Validates, resizes to 200px height (Lanczos3), stores as PNG
	// ============================================================

	file, _, err := r.FormFile("logo")
	if err == nil {
		defer file.Close()

		// Read all bytes — needed for content-type detection and processing
		rawBytes, err := io.ReadAll(file)
		if err != nil {
			log.Printf("logo read error: %v", err)
			h.App.Render(w, r, "profile.tmpl", map[string]any{
				"Profile":    p,
				"Error":      "Could not read uploaded file.",
				"Currencies": catalog.SupportedCurrencies,
			})
			return
		}

		// Process: validate, resize, encode to PNG
		processed, err := logo.Process(rawBytes)
		if err != nil {
			log.Printf("logo process error: %v", err)
			h.App.Render(w, r, "profile.tmpl", map[string]any{
				"Profile":    p,
				"Error":      "Invalid image: " + err.Error(),
				"Currencies": catalog.SupportedCurrencies,
			})
			return
		}

		// Store and get public URL
		publicURL, err := h.App.LogoStore.Save(user.ID, processed)
		if err != nil {
			log.Printf("logo store error: %v", err)
			http.Error(w, "Could not save logo", http.StatusInternalServerError)
			return
		}

		p.LogoURL = publicURL
		log.Printf("[logo] saved for user %d: %s", user.ID, p.LogoURL)
	}

	// ============================================================

	if p.Name == "" {
		h.App.Render(w, r, "profile.tmpl", map[string]any{
			"Profile":    p,
			"Error":      "Company name is required",
			"Currencies": catalog.SupportedCurrencies,
		})
		return
	}

	if err := h.App.BizRepo.Upsert(r.Context(), p); err != nil {
		log.Printf("profile upsert error: %v", err)

		h.App.Render(w, r, "profile.tmpl", map[string]any{
			"Profile":    p,
			"Error":      "Could not save profile",
			"Currencies": catalog.SupportedCurrencies,
		})
		
          	return
	}

	http.Redirect(w, r, "/profile?saved=true", http.StatusSeeOther)
}
