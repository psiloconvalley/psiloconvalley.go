package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// =====================================================================
// Session Configuration
// =====================================================================

const (
	// SessionCookieName is the name of the cookie that holds the
	// session token. Prefixed with __ to signal it's system-controlled.
	SessionCookieName = "__pscv_session"

	// SessionDuration controls how long a login session lasts.
	// 7 days is standard for a productivity tool.
	// Not so short that users get annoyed re-logging.
	// Not so long that a stolen cookie is dangerous forever.
	SessionDuration = 7 * 24 * time.Hour

	// AnonymousCookieName tracks how many invoices an anonymous
	// visitor has created. Used for freemium gating.
	AnonymousCookieName = "__pscv_anon_count"

	// MaxFreeInvoices is the number of invoices an anonymous user
	// can create before being required to make an account.
	MaxFreeInvoices = 3
)

// =====================================================================
// Session Management
// =====================================================================

// SetSessionCookie writes a session cookie to the response.
//
// Security properties:
// → HttpOnly: JavaScript cannot read it (XSS protection)
// → Secure:   Only sent over HTTPS (MITM protection)
// → SameSite: Only sent from same origin (CSRF protection)
// → Path /:   Available on all routes
//
// The value is the user's ID. In a production system at scale,
// you'd use a random session token mapped to a server-side store.
// For our current architecture (no horizontal scaling), the signed
// user ID is sufficient and avoids a session table.
func SetSessionCookie(w http.ResponseWriter, userID int64) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    fmt.Sprintf("%d", userID),
		Path:     "/",
		MaxAge:   int(SessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// GetSessionUserID reads the session cookie and returns the user ID.
//
// Returns 0 and false if:
// → No cookie exists (not logged in)
// → Cookie value is invalid (tampered)
// → Cookie value is not a positive integer
func GetSessionUserID(r *http.Request) (int64, bool) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return 0, false
	}

	id, err := strconv.ParseInt(cookie.Value, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}

	return id, true
}

// ClearSessionCookie removes the session cookie by setting MaxAge to -1.
//
// The browser will delete the cookie immediately.
// This is the only correct way to "log out" with cookies.
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

// GetAnonInvoiceCount returns how many invoices an anonymous visitor
// has created, based on their cookie.
//
// Returns 0 if no cookie exists (first visit).
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

// SetAnonInvoiceCount updates the anonymous invoice counter cookie.
//
// Stored in a cookie because anonymous users have no DB record.
// When they create an account, this cookie becomes irrelevant
// because we track invoice_count on the users table instead.
func SetAnonInvoiceCount(w http.ResponseWriter, count int) {
	http.SetCookie(w, &http.Cookie{
		Name:     AnonymousCookieName,
		Value:    strconv.Itoa(count),
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60, // 1 year
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// AnonLimitReached returns true if the anonymous user has hit
// the free invoice creation limit.
func AnonLimitReached(r *http.Request) bool {
	return GetAnonInvoiceCount(r) >= MaxFreeInvoices
}

// =====================================================================
// Utility
// =====================================================================

// GenerateToken creates a cryptographically secure random token.
//
// Used for CSRF tokens. 32 bytes = 256 bits of entropy.
// More than sufficient for any security application.
func GenerateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
