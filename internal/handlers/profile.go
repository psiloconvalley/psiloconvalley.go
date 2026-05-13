package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	// Logo Upload Handling
	// ============================================================

	file, header, err := r.FormFile("logo")
	if err == nil {
		defer file.Close()

		// Validate extension
		ext := strings.ToLower(filepath.Ext(header.Filename))

		allowed := map[string]bool{
			".png":  true,
			".jpg":  true,
			".jpeg": true,
			".webp": true,
			".svg":  true,
		}

		if !allowed[ext] {
			h.App.Render(w, r, "profile.tmpl", map[string]any{
				"Profile":    p,
				"Error":      "Unsupported image format. Use PNG, JPG, WEBP, or SVG.",
				"Currencies": catalog.SupportedCurrencies,
			})
			return
		}

		// Ensure upload directory exists
		uploadDir := filepath.Join("static", "uploads", "logos")

		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			log.Printf("mkdir error: %v", err)

			http.Error(w, "Could not prepare upload directory", http.StatusInternalServerError)
			return
		}

		// Stable filename per user
		filename := fmt.Sprintf("logo-user-%d%s", user.ID, ext)

		dstPath := filepath.Join(uploadDir, filename)

		dst, err := os.Create(dstPath)
		if err != nil {
			log.Printf("logo create error: %v", err)

			http.Error(w, "Could not save logo", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			log.Printf("logo write error: %v", err)

			http.Error(w, "Could not write logo", http.StatusInternalServerError)
			return
		}

		// Public URL
		p.LogoURL = "/static/uploads/logos/" + filename

		log.Printf("uploaded logo for user %d: %s", user.ID, p.LogoURL)
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
