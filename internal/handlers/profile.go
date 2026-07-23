package handlers

import (
	"database/sql"
	"errors"
	"io"
	"log/slog"
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
		slog.Error("profile fetch failed", "err", err)
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
   "LogoRemoved":     r.URL.Query().Get("logo_removed") == "true",
   "LogoError":       r.URL.Query().Get("logo_error") == "true",
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
   "BaseURL":         h.App.BaseURL,
})
}

func (h *Handlers) ProfilePost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Max upload size: 2MB
	if err := r.ParseMultipartForm(logo.MaxInputBytes); err != nil {
		slog.Error("profile multipart parse failed", "err", err)

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
		Country: catalog.NormalizeCountry(r.FormValue("country")),
		Phone:   catalog.FormatPhone(r.FormValue("phone")),
		TaxID:   strings.TrimSpace(r.FormValue("tax_id")),
		Currency: func() string {
			c := strings.TrimSpace(r.FormValue("currency"))
			if c == "" {
				return "USD"
			}
			return c
		}(),
		ZelleID:       strings.TrimSpace(r.FormValue("zelle_id")),
		VenmoHandle:   strings.TrimSpace(r.FormValue("venmo_handle")),
		CashAppHandle: strings.TrimSpace(r.FormValue("cashapp_handle")),
		ServiceAreas:  strings.TrimSpace(r.FormValue("service_areas")),
	}

	// Preserve existing logo by default
	if existing != nil {
		p.LogoURL = existing.LogoURL
		p.Slug = existing.Slug
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
			slog.Error("logo read failed", "err", err)
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
			slog.Error("logo process failed", "err", err)
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
			slog.Error("logo store failed", "err", err)
			http.Error(w, "Could not save logo", http.StatusInternalServerError)
			return
		}

		p.LogoURL = publicURL
		slog.Info("logo saved", "user_id", user.ID, "url", p.LogoURL)
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
		slog.Error("profile upsert failed", "err", err)

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
			slog.Error("language update failed", "err", err)
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
		slog.Error("password update failed", "err", err)
		http.Redirect(w, r, "/profile?pw_error=failed", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/profile?pw_saved=true", http.StatusSeeOther)
}
// ProfileLogoDeletePost removes a user's logo from storage and clears
// the logo_url from their business profile.
func (h *Handlers) ProfileLogoDeletePost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := h.App.LogoStore.Delete(user.ID); err != nil {
		slog.Error("logo delete failed", "user_id", user.ID, "err", err)
	}

	if err := h.App.BizRepo.ClearLogo(r.Context(), user.ID); err != nil {
		slog.Error("logo url clear failed", "user_id", user.ID, "err", err)
		http.Redirect(w, r, "/profile?logo_error=true", http.StatusSeeOther)
		return
	}

	slog.Info("logo removed", "user_id", user.ID)
	http.Redirect(w, r, "/profile?logo_removed=true", http.StatusSeeOther)
}

