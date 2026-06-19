// internal/handlers/passkey.go
package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-webauthn/webauthn/webauthn"

	"psiloconvalley/internal/audit"
	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/repo"
)

// =====================================================================
// WebAuthn User Adapter
// Makes repo.User satisfy the webauthn.User interface.
// =====================================================================

type webAuthnUser struct {
	User  *repo.User
	Creds []webauthn.Credential
}

func (u *webAuthnUser) WebAuthnID() []byte                         { return []byte(u.User.Email) }
func (u *webAuthnUser) WebAuthnName() string                       { return u.User.Email }
func (u *webAuthnUser) WebAuthnDisplayName() string                { return u.User.DisplayName() }
func (u *webAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.Creds }
func (u *webAuthnUser) WebAuthnIcon() string                       { return "" }

// =====================================================================
// Session Helpers — signed cookie storage for WebAuthn challenges
// =====================================================================

func setWebAuthnSession(w http.ResponseWriter, data *webauthn.SessionData) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	auth.SetSignedCookie(w, auth.WebAuthnSessionCookie, string(b), 300) // 5 minutes
	return nil
}

func getWebAuthnSession(r *http.Request) (*webauthn.SessionData, bool) {
	payload, ok := auth.GetSignedCookie(r, auth.WebAuthnSessionCookie)
	if !ok {
		return nil, false
	}
	var data webauthn.SessionData
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return nil, false
	}
	return &data, true
}

func clearWebAuthnSession(w http.ResponseWriter) {
	auth.ClearCookie(w, auth.WebAuthnSessionCookie)
}

// =====================================================================
// Registration — Add a Passkey (from Profile page)
// =====================================================================

// PasskeyRegistrationBegin generates WebAuthn options for the browser.
// The browser uses these to trigger Face ID / Touch ID.
func (h *Handlers) PasskeyRegistrationBegin(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	wUser := &webAuthnUser{User: user}

	options, sessionData, err := auth.WebAuthn.BeginRegistration(wUser)
	if err != nil {
		slog.Error("passkey registration begin failed",
			"user_id", user.ID, "err", err)
		http.Error(w, "Could not begin registration", http.StatusInternalServerError)
		return
	}

	if err := setWebAuthnSession(w, sessionData); err != nil {
		slog.Error("passkey session store failed",
			"user_id", user.ID, "err", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(options)
}

// PasskeyRegistrationFinish verifies the browser response and stores the credential.
func (h *Handlers) PasskeyRegistrationFinish(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Retrieve and clear the pending challenge
	sessionData, ok := getWebAuthnSession(r)
	if !ok {
		http.Error(w, "Session expired — please try again", http.StatusBadRequest)
		return
	}
	clearWebAuthnSession(w)

	wUser := &webAuthnUser{User: user}

	// Verify the browser's response against the challenge
	credential, err := auth.WebAuthn.FinishRegistration(wUser, *sessionData, r)
	if err != nil {
		slog.Warn("passkey registration finish failed",
			"user_id", user.ID, "err", err)
		http.Error(w, "Registration failed — please try again", http.StatusBadRequest)
		return
	}

	// Store the credential in the database
	cred := &repo.WebAuthnCredential{
		UserID:         user.ID,
		CredentialID:   credential.ID,
		PublicKey:      credential.PublicKey,
		SignCount:      uint32(credential.Authenticator.SignCount),
		DeviceName:     "Passkey",
		BackupEligible: credential.Flags.BackupEligible,
		BackupState:    credential.Flags.BackupState,
	}
	
	if err := h.App.PasskeyRepo.Create(r.Context(), cred); err != nil {
		slog.Error("passkey credential store failed",
			"user_id", user.ID, "err", err)
		http.Error(w, "Could not save passkey", http.StatusInternalServerError)
		return
	}

	slog.Info("passkey registered",
		"user_id", user.ID,
		"credential_id_len", len(credential.ID),
	)
	audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
		UserID:     audit.UserIDPtr(user.ID),
		Action:     audit.ActionPasskeyRegistered,
		EntityType: audit.EntityPasskey,
		IPAddress:  audit.IPFromRequest(r),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// =====================================================================
// Authentication — Use a Passkey (from Login page)
// =====================================================================

// PasskeyLoginBegin generates an assertion challenge for the browser.
func (h *Handlers) PasskeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	options, sessionData, err := auth.WebAuthn.BeginDiscoverableLogin()
	if err != nil {
		slog.Error("passkey login begin failed", "err", err)
		http.Error(w, "Could not begin login", http.StatusInternalServerError)
		return
	}

	if err := setWebAuthnSession(w, sessionData); err != nil {
		slog.Error("passkey login session store failed", "err", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(options)
}

// PasskeyLoginFinish verifies the assertion and creates a session if valid.
func (h *Handlers) PasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	sessionData, ok := getWebAuthnSession(r)
	if !ok {
		http.Error(w, "Session expired — please try again", http.StatusBadRequest)
		return
	}
	clearWebAuthnSession(w)

	var foundUser *repo.User
	var foundCred *repo.WebAuthnCredential

	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		cred, err := h.App.PasskeyRepo.GetByCredentialID(r.Context(), rawID)
		if err != nil {
			return nil, err
		}
		user, err := h.App.UserRepo.GetByID(cred.UserID)
		if err != nil {
			return nil, err
		}
		foundUser = user
		foundCred = cred

		wCred := webauthn.Credential{
			ID:        cred.CredentialID,
			PublicKey: cred.PublicKey,
			Flags: webauthn.CredentialFlags{
				BackupEligible: cred.BackupEligible,
				BackupState:    cred.BackupState,
			},
			Authenticator: webauthn.Authenticator{
				SignCount: uint32(cred.SignCount),
			},
		}
			return &webAuthnUser{User: user, Creds: []webauthn.Credential{wCred}}, nil
	}

	credential, err := auth.WebAuthn.FinishDiscoverableLogin(handler, *sessionData, r)
	if err != nil {
		slog.Warn("passkey login finish failed", "err", err)
		audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
			Action:     audit.ActionPasskeyLoginFailed,
			EntityType: audit.EntityPasskey,
			IPAddress:  audit.IPFromRequest(r),
		})
		http.Error(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	// Update sign count — detects cloned credentials
	if err := h.App.PasskeyRepo.UpdateSignCount(
		r.Context(),
		foundCred.CredentialID,
		uint32(credential.Authenticator.SignCount),
	); err != nil {
		slog.Warn("passkey sign count update failed",
			"user_id", foundUser.ID, "err", err)
	}

	// Issue session — identical to password login
	auth.SetSessionCookie(w, foundUser.ID)

	slog.Info("passkey login successful", "user_id", foundUser.ID)
	audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
		UserID:     audit.UserIDPtr(foundUser.ID),
		Action:     audit.ActionPasskeyLoginOK,
		EntityType: audit.EntityPasskey,
		IPAddress:  audit.IPFromRequest(r),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "ok",
		"redirect": "/dashboard",
	})
}
