package handlers

import (
	"database/sql"
	"errors"
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

	log.Printf("REGISTER ATTEMPT: %s from %s", email, r.RemoteAddr)

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

	log.Printf("LOGIN ATTEMPT: %s from %s", email, r.RemoteAddr)

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
	if !user.CheckPassword(pass) {
		h.App.Render(w, r, "login.tmpl", map[string]any{"Error": "Invalid credentials", "Email": email})
		return
	}
	auth.SetSessionCookie(w, user.ID)
	http.Redirect(w, r, "/tools", http.StatusSeeOther)
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
			log.Printf("OAuth state mismatch — redirecting to retry")
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
	log.Printf("GOOGLE AUTH: %s (id=%d, new=%v)", user.Email, user.ID, isNew)

	if isNew {
		http.Redirect(w, r, "/profile?welcome=true", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/invoices", http.StatusSeeOther)
}
