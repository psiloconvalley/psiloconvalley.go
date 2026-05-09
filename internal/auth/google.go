// internal/auth/google.go
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// =====================================================================
// Config
// =====================================================================

var googleOAuthConfig *oauth2.Config

func InitGoogleOAuth() {
	googleOAuthConfig = &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

func GoogleOAuthEnabled() bool {
	return os.Getenv("GOOGLE_CLIENT_ID") != ""
}

// =====================================================================
// State Cookie - CSRF Protection For OAuth Flow
// =====================================================================

const oauthStateCookie = "__pscv_oauth_state"

func setOAuthStateCookie(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes - just enough to complete OAuth
		HttpOnly: true,
		Secure:   os.Getenv("RAILWAY_ENVIRONMENT") != "",
		SameSite: http.SameSiteLaxMode,
	})
}

func verifyOAuthStateCookie(r *http.Request, state string) bool {
	cookie, err := r.Cookie(oauthStateCookie)
	if err != nil {
		return false
	}
	return cookie.Value == state && state != ""
}

func clearOAuthStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

func generateStateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// =====================================================================
// Google User Info
// =====================================================================

type GoogleUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func fetchGoogleUser(ctx context.Context, token *oauth2.Token) (*GoogleUser, error) {
	client := googleOAuthConfig.Client(ctx, token)

	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google api status %d", resp.StatusCode)
	}

	var gu GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&gu); err != nil {
		return nil, fmt.Errorf("decode userinfo: %w", err)
	}

	if gu.Email == "" {
		return nil, fmt.Errorf("no email returned from Google")
	}

	if !gu.VerifiedEmail {
		return nil, fmt.Errorf("google email not verified")
	}

	return &gu, nil
}

// =====================================================================
// Handlers
// =====================================================================

// GoogleLoginHandler redirects user to Google consent screen
func GoogleLoginHandler(w http.ResponseWriter, r *http.Request) {
	if !GoogleOAuthEnabled() {
		http.Error(w, "Google OAuth not configured", http.StatusNotImplemented)
		return
	}

	state, err := generateStateToken()
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	setOAuthStateCookie(w, state)

	url := googleOAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// ProcessGoogleCallback handles the redirect back from Google
func ProcessGoogleCallback(w http.ResponseWriter, r *http.Request) (*GoogleUser, error) {
	// Verify state matches what we set (CSRF check)
	state := r.URL.Query().Get("state")
	if !verifyOAuthStateCookie(r, state) {
		return nil, fmt.Errorf("invalid oauth state")
	}
	clearOAuthStateCookie(w)

	// User denied access
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		return nil, fmt.Errorf("oauth denied: %s", errParam)
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		return nil, fmt.Errorf("no code in callback")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	token, err := googleOAuthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	return fetchGoogleUser(ctx, token)
}
