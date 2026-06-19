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

u, _ := h.App.UserRepo.GetByID(user.ID)
isMagicLogin := r.URL.Query().Get("magic") == "true"

passkeys, _ := h.App.PasskeyRepo.GetByUserID(r.Context(), user.ID)

h.App.Render(w, r, "profile.tmpl", map[string]any{
   "Profile":         profile,
   "Saved":           r.URL.Query().Get("saved") == "true",
   "Welcome":         r.URL.Query().Get("welcome") == "true",
   "Currencies":      catalog.SupportedCurrencies,
   "StripeConnected": r.URL.Query().Get("stripe_connected") == "1",
   "StripeError":     r.URL.Query().Get("stripe_error") == "1",
   "MagicLogin":      isMagicLogin,
   "IsGoogleUser":    u != nil && u.IsGoogleUser(),
   "HasPassword":     u != nil && u.PasswordHash != "",
   "PasswordSaved":   r.URL.Query().Get("pw_saved") == "true",
   "PasswordError":   r.URL.Query().Get("pw_error"),
   "Passkeys":        passkeys,
   "PasskeyDeleted":  r.URL.Query().Get("passkey_deleted") == "true",
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
		State:   catalog.NormalizeState(r.FormValue("state")),
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

	// ── Save language preference ─────────────────────────────────────
	lang := strings.TrimSpace(r.FormValue("language"))
	if lang != "" {
		if err := h.App.UserRepo.UpdateLanguage(r.Context(), user.ID, lang); err != nil {
			log.Printf("language update error: %v", err)
		}
	}

	http.Redirect(w, r, "/profile?saved=true", http.StatusSeeOther)

}

func (h *Handlers) ChangePasswordPost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	current := r.FormValue("current_password")
	newPass := r.FormValue("new_password")
	confirm := r.FormValue("confirm_password")

	// Reload user to get current password hash
	u, err := h.App.UserRepo.GetByID(user.ID)
	if err != nil {
		http.Error(w, "Could not load account", http.StatusInternalServerError)
		return
	}

	// If the user has no password yet (magic link / Google OAuth arrival),
	// skip current-password check — there is nothing to verify against.
	hasPassword := u.PasswordHash != ""

	if hasPassword {
		if !u.CheckPassword(current) {
			http.Redirect(w, r, "/profile?pw_error=current", http.StatusSeeOther)
			return
		}
	}

	if len(newPass) < 8 {
		http.Redirect(w, r, "/profile?pw_error=short", http.StatusSeeOther)
		return
	}
	if newPass != confirm {
		http.Redirect(w, r, "/profile?pw_error=mismatch", http.StatusSeeOther)
		return
	}

	if err := h.App.UserRepo.UpdatePassword(r.Context(), user.ID, newPass); err != nil {
		log.Printf("[profile] password update error: %v", err)
		http.Redirect(w, r, "/profile?pw_error=failed", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/profile?pw_saved=true", http.StatusSeeOther)
}
