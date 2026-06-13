// internal/handlers/middleware.go
package handlers

import (
	"net/http"
)

// SecurityHeaders applies defensive HTTP headers to every response.
//
// Headers applied:
//
//	X-Content-Type-Options       — prevents MIME sniffing attacks
//	X-Frame-Options              — blocks clickjacking via iframes
//	X-XSS-Protection             — legacy XSS filter for older browsers
//	Referrer-Policy              — controls referrer header leakage
//	Strict-Transport-Security    — forces HTTPS for 2 years, includes subdomains
//	Permissions-Policy           — disables browser features we don't use
//	Cross-Origin-Opener-Policy   — isolates browsing context (Spectre mitigation)
//	Cross-Origin-Resource-Policy — controls cross-origin resource reads
//	Content-Security-Policy      — restricts resource origins
//
// CSP notes:
//
//	img-src https:   — allows logos stored on Supabase/CDN (user-supplied,
//	                   validated at upload time — see FIX D4)
//	script-src       — unsafe-inline required for inline JS in templates.
//	                   Future: migrate to nonce-based CSP.
//	frame-src        — Stripe Elements renders in an iframe
//	connect-src      — Stripe.js makes XHR calls to api.stripe.com
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// ── Prevent MIME sniffing ─────────────────────────────────────
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// ── Block clickjacking ────────────────────────────────────────
		w.Header().Set("X-Frame-Options", "DENY")

		// ── Legacy XSS filter (older browsers) ───────────────────────
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// ── Referrer leakage control ──────────────────────────────────
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// ── Force HTTPS for 2 years, include subdomains ───────────────
		// preload: opts in to browser HSTS preload lists
		w.Header().Set("Strict-Transport-Security",
			"max-age=63072000; includeSubDomains; preload")

		// ── Disable browser features we never use ─────────────────────
		w.Header().Set("Permissions-Policy",
			"camera=(), microphone=(), geolocation=(), payment=(), "+
				"usb=(), magnetometer=(), gyroscope=(), accelerometer=()")

		// ── Browsing context isolation (Spectre mitigation) ───────────
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")

		// ── Cross-origin resource read control ────────────────────────
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")

		// ── Content Security Policy ───────────────────────────────────
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+

				// Styles: inline allowed (template styles), Google Fonts
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+

				// Fonts: Google Fonts CDN only
				"font-src 'self' https://fonts.gstatic.com; "+

				// Scripts: inline allowed (template JS) + Stripe.js
				// Future: replace unsafe-inline with nonce-based CSP
				"script-src 'self' 'unsafe-inline' https://js.stripe.com; "+

				// Images: self, data URIs, all HTTPS (logos on Supabase/CDN)
				// FIX D4: https: required for external logo URLs
				"img-src 'self' data: https:; "+

				// Stripe Elements renders inside iframes
				"frame-src https://js.stripe.com https://hooks.stripe.com; "+

				// Stripe.js makes XHR calls to api.stripe.com
				"connect-src 'self' https://api.stripe.com; "+

				// Upgrade any accidental HTTP requests to HTTPS
				"upgrade-insecure-requests;",
		)

		next.ServeHTTP(w, r)
	})
}
