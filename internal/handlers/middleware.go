package handlers

import (
	"net/http"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/repo"
)

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; script-src 'self' 'unsafe-inline'; img-src 'self' data:;")
		next.ServeHTTP(w, r)
	})
}

func CanAccessInvoice(r *http.Request, inv *repo.Invoice) bool {
	if inv.UserID == nil {
		return true
	}
	user := auth.GetUser(r)
	if user == nil {
		return false
	}
	return user.ID == *inv.UserID
}
