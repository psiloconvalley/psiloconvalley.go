// internal/handlers/invoice_view_log.go
//
// Hook helper for logging invoice views. Called from InvoiceDetail,
// InvoicePDFGet, and InvoicePayGet after the access check succeeds.
//
// Design:
//   - No-op when ViewLoggingEnabled flag is false (zero cost at the call site).
//   - Owner views (authenticated or anon-cookie match) are never logged.
//   - Bot UAs are logged separately so they can be filtered from the UI.
//   - DB write happens in a goroutine with defer recover() — never blocks
//     the HTTP response, never surfaces errors to the user.
//
// The goroutine uses context.Background() rather than r.Context() so that
// the write survives the response completing. r.Context() is cancelled the
// moment the handler returns, which would race the goroutine.
package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"psiloconvalley/internal/analytics"
	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/repo"
)

// logViewTimeout caps the async DB write so a slow database can't leak
// goroutines indefinitely under load.
const logViewTimeout = 3 * time.Second

// logInvoiceViewAsync classifies the request and, if appropriate, enqueues
// a background goroutine that writes a row to invoice_views.
//
// Safe to call unconditionally — returns immediately when the feature flag
// is off or when the viewer is the owner. Never panics, never blocks.
func (h *Handlers) logInvoiceViewAsync(r *http.Request, inv *repo.Invoice, eventType string) {
	if !h.App.ViewLoggingEnabled {
		return
	}
	if inv == nil {
		return
	}

	// Classify: is this the owner, an anon-cookie match, or a stranger?
	user := auth.GetUser(r)
	isOwner := inv.UserID != nil && user != nil && user.ID == *inv.UserID

	isAnonMatch := false
	if inv.AnonymousToken != "" {
		if anonToken, ok := auth.GetAnonymousToken(r); ok && anonToken == inv.AnonymousToken {
			isAnonMatch = true
		}
	}

	ua := r.Header.Get("User-Agent")
	decision := analytics.DecideView(isOwner, isAnonMatch, ua)
	if !decision.ShouldLog {
		return
	}

	// Snapshot everything the goroutine will need. Do NOT reference r or
	// anything derived from it inside the goroutine — the request may be
	// torn down the instant this handler returns.
	ip := clientIP(r)
	referer := r.Header.Get("Referer")

	view := repo.InvoiceView{
		InvoiceID:  inv.ID,
		EventType:  eventType,
		ViewerType: decision.ViewerType,
		IPAddress:  nullableString(ip),
		UserAgent:  nullableString(ua),
		Referer:    nullableString(referer),
	}

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Warn("invoice view logging panicked",
					"invoice_id", view.InvoiceID,
					"event", view.EventType,
					"panic", rec,
				)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), logViewTimeout)
		defer cancel()

		if err := h.App.InvoiceViewRepo.Log(ctx, view); err != nil {
			slog.Warn("invoice view logging failed",
				"invoice_id", view.InvoiceID,
				"event", view.EventType,
				"viewer_type", view.ViewerType,
				"err", err,
			)
		}
	}()
}

// clientIP extracts the best-effort client IP from the request. Prefers
// X-Forwarded-For (Railway sets this), falls back to RemoteAddr.
//
// Note: X-Forwarded-For can be spoofed by direct clients. Behind Railway's
// proxy it is trustworthy. We log it for operator diagnostics, not for
// security decisions.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// XFF can be a comma-separated list; the first entry is the client.
		if comma := strings.Index(xff, ","); comma >= 0 {
			return strings.TrimSpace(xff[:comma])
		}
		return strings.TrimSpace(xff)
	}
	// RemoteAddr is host:port; strip the port.
	addr := r.RemoteAddr
	if colon := strings.LastIndex(addr, ":"); colon >= 0 {
		return addr[:colon]
	}
	return addr
}

// nullableString returns nil for empty strings so the DB stores NULL
// instead of empty string. Keeps column semantics clean.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
