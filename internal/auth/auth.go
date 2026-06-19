package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// =====================================================================
// Session Configuration
// =====================================================================

const (
	SessionCookieName       = "__pscv_session"
	SessionDuration         = 7 * 24 * time.Hour
	IdleTimeout             = 30 * time.Minute
	AnonymousCookieName     = "__pscv_anon_count"
	AnonymousTokenCookie    = "__pscv_anon_token"
	WebAuthnSessionCookie   = "__pscv_wa_session"
	MaxFreeInvoices         = 3
)


// =====================================================================
// Session Secret — HMAC-SHA256 Signing
// =====================================================================

var sessionSecret []byte

// InitSessionSecret loads the SESSION_SECRET env var.
// Must be called once at startup before any session operations.
// Panics if the secret is missing or too short — this is intentional.
func InitSessionSecret() {
	secret := os.Getenv("SESSION_SECRET")
	if len(secret) < 32 {
		log.Fatal("[auth] FATAL: SESSION_SECRET env var must be at least 32 characters")
	}
	sessionSecret = []byte(secret)
}

// sign produces an HMAC-SHA256 signature for the given payload.
func sign(payload string) string {
	mac := hmac.New(sha256.New, sessionSecret)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifySignature checks an HMAC-SHA256 signature in constant time.
func verifySignature(payload, signature string) bool {
	expected := sign(payload)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// =====================================================================
// Session Management — Signed Cookies
// =====================================================================

// SetSessionCookie creates an HMAC-signed session cookie.
// Cookie value format: userID.unixTimestamp.hmacSignature
func SetSessionCookie(w http.ResponseWriter, userID int64) {
	ts := time.Now().Unix()
	payload := fmt.Sprintf("%d.%d", userID, ts)
	sig := sign(payload)
	value := fmt.Sprintf("%s.%s", payload, sig)

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(SessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// GetSessionUserID extracts and validates the user ID from a signed session cookie.
// Returns (0, false) if the cookie is missing, malformed, expired, or tampered.
func GetSessionUserID(r *http.Request) (int64, bool) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return 0, false
	}

	// Expected format: userID.timestamp.signature
	parts := strings.SplitN(cookie.Value, ".", 3)
	if len(parts) != 3 {
		return 0, false
	}

	userIDStr, tsStr, sig := parts[0], parts[1], parts[2]

	// Verify HMAC signature — constant time
	payload := userIDStr + "." + tsStr
	if !verifySignature(payload, sig) {
		return 0, false
	}

	// Parse user ID
	id, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}

	// Check timestamp hasn't expired
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return 0, false
	}
	issued := time.Unix(ts, 0)
	if time.Since(issued) > IdleTimeout {
		return 0, false
	}

	return id, true
}


// RefreshSessionCookie re-issues the session cookie with a fresh timestamp.
// Called on every authenticated request to implement idle timeout —
// active users stay logged in, idle users are eventually logged out.
func RefreshSessionCookie(w http.ResponseWriter, userID int64) {
	SetSessionCookie(w, userID)
}
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// =====================================================================
// Anonymous Invoice Counter
// =====================================================================

func GetAnonInvoiceCount(r *http.Request) int {
	cookie, err := r.Cookie(AnonymousCookieName)
	if err != nil {
		return 0
	}
	count, err := strconv.Atoi(cookie.Value)
	if err != nil || count < 0 {
		return 0
	}
	return count
}

func SetAnonInvoiceCount(w http.ResponseWriter, count int) {
	http.SetCookie(w, &http.Cookie{
		Name:     AnonymousCookieName,
		Value:    strconv.Itoa(count),
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func AnonLimitReached(r *http.Request) bool {
	return GetAnonInvoiceCount(r) >= MaxFreeInvoices
}

// =====================================================================
// Anonymous Ownership Token
// =====================================================================

func GetAnonymousToken(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(AnonymousTokenCookie)
	if err != nil {
		return "", false
	}
	if cookie.Value == "" {
		return "", false
	}
	return cookie.Value, true
}

func SetAnonymousToken(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     AnonymousTokenCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearAnonymousToken(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     AnonymousTokenCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// =====================================================================
// Utility
// =====================================================================
// SetSignedCookie stores a signed cookie value using HMAC-SHA256.
// Value format: base64(payload).hex(hmac)
func SetSignedCookie(w http.ResponseWriter, name string, payload string, maxAgeSeconds int) {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(payload))
	sig := sign(encoded)
	value := encoded + "." + sig

	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAgeSeconds,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// GetSignedCookie verifies and returns the original payload.
// Returns ("", false) if missing, malformed, or tampered.
func GetSignedCookie(r *http.Request, name string) (string, bool) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return "", false
	}

	parts := strings.SplitN(cookie.Value, ".", 2)
	if len(parts) != 2 {
		return "", false
	}

	encoded, sig := parts[0], parts[1]
	if !verifySignature(encoded, sig) {
		return "", false
	}

	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", false
	}

	return string(raw), true
}

// ClearCookie expires a cookie immediately.
func ClearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}


func GenerateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
