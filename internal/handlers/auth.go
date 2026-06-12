package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"psiloconvalley/internal/auth"
)

func (h *Handlers) RegisterGet(w http.ResponseWriter, r *http.Request) {
	h.App.Render(w, r, "register.tmpl", nil)
}

func (h *Handlers) RegisterPost(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	pass := r.FormValue("password")
	confirm := r.FormValue("confirm_password")

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
		log.Printf("login db error: %v", err)
		http.Error(w, "Auth service unavailable", http.StatusInternalServerError)
		return
	}
	// If a previous lockout has expired, clear it so the user gets
	// a fresh set of attempts instead of being re-locked immediately.
	if user.LockedUntil != nil && !user.IsLocked() {
		if err := h.App.UserRepo.ResetFailedLogins(r.Context(), user.ID); err != nil {
			log.Printf("[auth] reset expired lockout error for user %d: %v", user.ID, err)
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
			log.Printf("[auth] record failed login error for user %d: %v", user.ID, err)
		}
		h.App.Render(w, r, "login.tmpl", map[string]any{"Error": "Invalid credentials", "Email": email})
		return
	}

	// Successful login — reset failed attempts.
	if err := h.App.UserRepo.ResetFailedLogins(r.Context(), user.ID); err != nil {
		log.Printf("[auth] reset failed logins error for user %d: %v", user.ID, err)
	}

	// Transparent rehash — upgrade bcrypt to Argon2id on next login.
	if user.NeedsRehash() {
		if err := h.App.UserRepo.RehashPassword(r.Context(), user.ID, pass); err != nil {
			log.Printf("[auth] rehash failed for user %d: %v", user.ID, err)
		}
	}

	auth.SetSessionCookie(w, user.ID)
	http.Redirect(w, r, "/invoices", http.StatusSeeOther)
}
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	auth.ClearSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	googleUser, err := auth.ProcessGoogleCallback(w, r)
	if err != nil {
		log.Printf("google oauth callback error: %v", err)
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
		log.Printf("google user upsert error: %v", err)
		http.Error(w, "Could not create or find account", http.StatusInternalServerError)
		return
	}

	auth.SetSessionCookie(w, user.ID)

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
		log.Printf("[auth] magic token error for %s: %v", email, err)
		h.App.Render(w, r, "forgot_password.tmpl", map[string]any{"Success": successMsg})
		return
	}

	// Only send email if token was created (email exists + not in cooldown)
	if token != "" {
		link := h.App.BaseURL + "/auth/magic?token=" + token
		if err := h.App.Mailer.SendMagicLink(email, link); err != nil {
			log.Printf("[auth] magic link email failed for %s: %v", email, err)
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
		log.Printf("[auth] invalid magic token attempt: %v", err)
		h.App.Render(w, r, "forgot_password.tmpl", map[string]any{
			"Error": "This link is invalid or has expired. Please request a new one.",
		})
		return
	}

	auth.SetSessionCookie(w, user.ID)
	http.Redirect(w, r, "/profile?magic=true", http.StatusSeeOther)
}
