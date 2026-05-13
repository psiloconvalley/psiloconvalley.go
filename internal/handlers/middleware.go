// internal/handlers/middleware.go
package handlers

import (
	"net/http"
)

// SecurityHeaders applies defensive HTTP headers to every response.
//
// FIX D4: img-src now includes https: to allow external logo URLs stored
// in business_profiles.logo_url. Previously any logo hosted on S3,
// Cloudflare, or any CDN was silently blocked by the CSP, rendering blank
// on the invoice. The broader https: allowlist is acceptable here because
// logo URLs are user-supplied and already validated at upload time.
// If you later restrict to a specific CDN, replace https: with the exact
// origin (e.g., https://cdn.yourdomain.com).
//
// FIX D1: The exported CanAccessInvoice function has been removed.
// It was identical to the unexported canAccessInvoice method on Handlers
// (in limits.go) and was never called anywhere. Two identical functions
// with different receivers is a maintenance trap — one source of truth.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
				"font-src https://fonts.gstatic.com; "+
				"script-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: https:;") // FIX D4
		next.ServeHTTP(w, r)
	})
}
