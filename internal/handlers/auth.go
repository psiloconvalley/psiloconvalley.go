package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"strconv"
	"time"

	"psiloconvalley/internal/audit"
	"psiloconvalley/internal/auth"
)

func (h *Handlers) RegisterGet(w http.ResponseWriter, r *http.Request) {
	h.App.Render(w, r, "register.tmpl", nil)
}

func (h *Handlers) RegisterPost(w http.ResponseWriter, r *http.Request) {
	// ── Bot protection ──────────────────────────────────────────────
	// Layer 1: Honeypot — bots auto-fill hidden fields, humans don't
	if r.FormValue("full_name_honey") != "" {
		slog.Warn("registration blocked by honeypot", "ip", r.RemoteAddr)
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	// Layer 2: Timing — bots submit instantly, humans take 3+ seconds
	if bt := r.FormValue("bt"); bt != "" {
		if ts, err := strconv.ParseInt(bt, 10, 64); err == nil {
			elapsed := time.Now().UnixMilli() - ts
			if elapsed < 2000 {
				slog.Warn("registration blocked by timing check", "elapsed_ms", elapsed, "ip", r.RemoteAddr)
				http.Redirect(w, r, "/register", http.StatusSeeOther)
				return
			}
		}
	} else {
		// No JS execution — likely a bot
		slog.Warn("registration blocked, no JS token", "ip", r.RemoteAddr)
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	pass := r.FormValue("password")
	confirm := r.FormValue("confirm_password")

	// Layer 3: Disposable email domain blocklist
	// Loaded once from embedded file — O(1) lookup, zero network cost.
	if auth.IsDisposableEmail(email) {
		slog.Warn("registration blocked, disposable email domain", "ip", r.RemoteAddr)
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	if email == "" || !strings.Contains(email, "@") {
		h.App.Render(w, r, "register.tmpl", map[string]any{"Error": "Please enter a valid email address", "Email": email})
		return
	}
	if len(pass) < 8 {
		h.App.Render(w, r, "register.tmpl", map[string]any{"Error": "Password must be at least 8 characters", "Email": email})
		return
	}
	if pass != confirm {
		h.App.Render(w, r, "register.tmpl", map[string]any{"Error": "Passwords do not match", "Email": email})
		return
	}

	id, err := h.App.UserRepo.Create(email, pass)
	if err != nil {
		h.App.Render(w, r, "register.tmpl", map[string]any{"Error": "An account with this email already exists", "Email": email})
		return
	}
	auth.SetSessionCookie(w, id)
	audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
		UserID:    audit.UserIDPtr(id),
		Action:    audit.ActionAuthRegister,
		EntityType: audit.EntityUser,
		EntityID:  audit.EntityIDPtr(id),
		IPAddress: audit.IPFromRequest(r),
		Metadata:  map[string]any{"email": email},
	})
	http.Redirect(w, r, "/profile?welcome=true", http.StatusSeeOther)
}

func (h *Handlers) LoginGet(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{}
	switch r.URL.Query().Get("error") {
	case "oauth_failed":
		data["Error"] = "Google sign-in failed. Please try again or use email."
	case "oauth_denied":
		data["Error"] = "Google sign-in was cancelled."
	}
	if r.URL.Query().Get("reason") == "limit" {
		data["LimitBanner"] = true
	}
	h.App.Render(w, r, "login.tmpl", data)
}
func (h *Handlers) LoginPost(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	pass := r.FormValue("password")

	user, err := h.App.UserRepo.GetByEmail(email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.App.Render(w, r, "login.tmpl", map[string]any{"Error": "Invalid credentials", "Email": email})
			return
		}
		slog.Error("login db lookup failed", "err", err)
		http.Error(w, "Auth service unavailable", http.StatusInternalServerError)
		return
	}
	// If a previous lockout has expired, clear it so the user gets
	// a fresh set of attempts instead of being re-locked immediately.
	if user.LockedUntil != nil && !user.IsLocked() {
		if err := h.App.UserRepo.ResetFailedLogins(r.Context(), user.ID); err != nil {
			slog.Warn("lockout reset failed", "err", err, "user_id", user.ID)
		
		} else {
			user.FailedLoginAttempts = 0
			user.LockedUntil = nil
		}
	}

	// Check account lockout before attempting password verification.
	if user.IsLocked() {
		h.App.Render(w, r, "login.tmpl", map[string]any{
			"Error": fmt.Sprintf("Account temporarily locked. Try again in %d minutes.", user.LockoutRemaining()),
			"Email": email,
		})
		return
	}

	if !user.CheckPassword(pass) {
		// Record failed attempt — may trigger lockout.
		if err := h.App.UserRepo.RecordFailedLogin(r.Context(), user.ID); err != nil {
			slog.Warn("failed login recording error", "err", err, "user_id", user.ID)
		}
		audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
		UserID:     audit.UserIDPtr(user.ID),
		Action:     audit.ActionAuthLoginFailed,
		EntityType: audit.EntityUser,
		EntityID:   audit.EntityIDPtr(user.ID),
		IPAddress:  audit.IPFromRequest(r),
		Metadata:   map[string]any{"email": email},
	})
		h.App.Render(w, r, "login.tmpl", map[string]any{"Error": "Invalid credentials", "Email": email})
		return
	}

	// Successful login — reset failed attempts.
	if err := h.App.UserRepo.ResetFailedLogins(r.Context(), user.ID); err != nil {
		slog.Warn("failed login counter reset error", "err", err, "user_id", user.ID)
	}

	// Transparent rehash — upgrade bcrypt to Argon2id on next login.
	if user.NeedsRehash() {
		if err := h.App.UserRepo.RehashPassword(r.Context(), user.ID, pass); err != nil {
			slog.Warn("password rehash failed", "err", err, "user_id", user.ID)
		}
	}

	auth.SetSessionCookie(w, user.ID)
	audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
		UserID:     audit.UserIDPtr(user.ID),
		Action:     audit.ActionAuthLogin,
		EntityType: audit.EntityUser,
		EntityID:   audit.EntityIDPtr(user.ID),
		IPAddress:  audit.IPFromRequest(r),
		Metadata:   map[string]any{"email": email},
	})
	http.Redirect(w, r, "/invoices", http.StatusSeeOther)
}
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	auth.ClearSessionCookie(w)
	user := auth.GetUser(r)
	if user != nil {
		audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
			UserID:     audit.UserIDPtr(user.ID),
			Action:     audit.ActionAuthLogout,
			EntityType: audit.EntityUser,
			EntityID:   audit.EntityIDPtr(user.ID),
			IPAddress:  audit.IPFromRequest(r),
		})
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	googleUser, err := auth.ProcessGoogleCallback(w, r)
	if err != nil {
		slog.Error("google oauth callback failed", "err", err)
		if strings.Contains(err.Error(), "invalid oauth state") {
			http.Redirect(w, r, "/auth/google", http.StatusTemporaryRedirect)
			return
		}
		errMsg := "oauth_failed"
		if strings.Contains(err.Error(), "denied") {
			errMsg = "oauth_denied"
		}
		http.Redirect(w, r, "/login?error="+errMsg, http.StatusSeeOther)
		return
	}

	user, isNew, err := h.App.UserRepo.FindOrCreateGoogleUser(
		googleUser.Email, googleUser.ID, googleUser.Name, googleUser.Picture,
	)
	if err != nil {
		slog.Error("google user upsert failed", "err", err)
		http.Error(w, "Could not create or find account", http.StatusInternalServerError)
		return
	}

	auth.SetSessionCookie(w, user.ID)
		audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
		UserID:     audit.UserIDPtr(user.ID),
		Action:     audit.ActionAuthLogin,
		EntityType: audit.EntityUser,
		EntityID:   audit.EntityIDPtr(user.ID),
		IPAddress:  audit.IPFromRequest(r),
		Metadata:   map[string]any{"method": "google", "is_new": isNew},
	})

	if isNew {
		http.Redirect(w, r, "/profile?welcome=true", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/invoices", http.StatusSeeOther)
}

// ForgotPasswordGet shows the forgot password form.
func (h *Handlers) ForgotPasswordGet(w http.ResponseWriter, r *http.Request) {
	h.App.Render(w, r, "forgot_password.tmpl", nil)
}

// ForgotPasswordPost sends a magic link to the email if it exists.
// Always shows the same success message — never reveals if email exists.
func (h *Handlers) ForgotPasswordPost(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))

	const successMsg = "If that email is registered, you'll receive a login link shortly."

	if email == "" || !strings.Contains(email, "@") {
		h.App.Render(w, r, "forgot_password.tmpl", map[string]any{
			"Error": "Please enter a valid email address.",
		})
		return
	}

	// Generate and store token — silently ignore if email not found
	token, err := h.App.UserRepo.SetMagicToken(r.Context(), email)
	if err != nil {
		// Do not reveal the error — log it and show generic success
		slog.Error("magic link token generation failed", "err", err, "email", email)
		h.App.Render(w, r, "forgot_password.tmpl", map[string]any{"Success": successMsg})
		return
	}

	// Only send email if token was created (email exists + not in cooldown)
	if token != "" {
		link := h.App.BaseURL + "/auth/magic?token=" + token
		if err := h.App.Mailer.SendMagicLink(email, link); err != nil {
			slog.Error("magic link email send failed", "err", err, "email", email)
		}
	}

	h.App.Render(w, r, "forgot_password.tmpl", map[string]any{"Success": successMsg})
}
// MagicLinkGet shows a confirmation page before consuming the token.
// This prevents email scanners/prefetchers from burning one-time links.
func (h *Handlers) MagicLinkGet(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		http.Redirect(w, r, "/forgot-password", http.StatusSeeOther)
		return
	}

	h.App.Render(w, r, "magic_confirm.tmpl", map[string]any{
		"Token": token,
	})
}

// MagicLinkPost consumes the token, clears the old password,
// logs the user in, and sends them to set a new password.
func (h *Handlers) MagicLinkPost(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.FormValue("token"))
	if token == "" {
		http.Redirect(w, r, "/forgot-password", http.StatusSeeOther)
		return
	}

	user, err := h.App.UserRepo.ConsumeMagicToken(r.Context(), token)
	if err != nil {
		slog.Warn("invalid magic token attempt", "err", err)
		h.App.Render(w, r, "forgot_password.tmpl", map[string]any{
			"Error": "This link is invalid or has expired. Please request a new one.",
		})
		return
	}

	auth.SetSessionCookie(w, user.ID)
	http.Redirect(w, r, "/profile?magic=true", http.StatusSeeOther)
}
