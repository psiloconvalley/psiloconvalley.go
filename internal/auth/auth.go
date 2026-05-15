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
	SessionCookieName   = "__pscv_session"
	SessionDuration     = 7 * 24 * time.Hour
	AnonymousCookieName = "__pscv_anon_count"
	AnonymousTokenCookie = "__pscv_anon_token"
	MaxFreeInvoices     = 3
)

// =====================================================================
// Session Management
// =====================================================================

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

func GenerateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
