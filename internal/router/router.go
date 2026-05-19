// internal/router/router.go
package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/handlers"
)

func NewRouter(h *handlers.Handlers) http.Handler {
	r := chi.NewRouter()

	r.Use(handlers.SecurityHeaders)
	r.Use(auth.LoadUser(h.App.UserRepo))

	r.Handle("/static/*", http.StripPrefix("/static/",
		http.FileServer(http.Dir("static"))))

	// ── Public routes (no login required) ──────────────────────────
	r.Get("/", h.Index)
	r.Get("/healthz", h.Health)
	r.Get("/research", h.Research)
	r.Get("/tools", h.ToolsHub)
	r.Get("/pricing", h.PricingGet)
	r.Post("/stripe/webhook", h.StripeWebhook)
	r.Post("/feedback", h.Feedback)

	r.Get("/register", h.RegisterGet)
	r.Post("/register", h.RegisterPost)

	r.Get("/login", h.LoginGet)
	r.Post("/login", h.LoginPost)

	r.Post("/logout", h.Logout)

	r.Get("/auth/google", auth.GoogleLoginHandler)
	r.Get("/auth/google/callback", h.GoogleCallback)

	// ── Freemium invoice routes (ownership enforced in handlers) ───
	r.Get("/invoices/new", h.InvoiceNewGet)
	r.Post("/invoices/create", h.InvoiceCreatePost)
	r.Get("/invoices/{id}", h.InvoiceDetail)
	r.Get("/invoices/{id}/pdf", h.InvoicePDFGet)

	// ── Protected routes (login required) ──────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/checkout", h.CheckoutPost)
		r.Post("/billing/portal", h.BillingPortalPost)

		r.Get("/profile", h.ProfileGet)
		r.Post("/profile", h.ProfilePost)

		r.Get("/clients", h.ClientsList)
		r.Get("/clients/new", h.ClientNewGet)
		r.Post("/clients/new", h.ClientNewPost)

		// Invoice management (requires login)
		r.Get("/invoices", h.InvoicesList)
		r.Get("/invoices/{id}/edit", h.InvoiceEditGet)
		r.Post("/invoices/{id}/edit", h.InvoiceUpdatePost)
		r.Post("/invoices/{id}/status", h.InvoiceStatusPost)
		r.Get("/invoices/{id}/send", h.InvoiceSendGet)
		r.Post("/invoices/{id}/send", h.InvoiceSendPost)
		r.Get("/invoices/{id}/duplicate", h.InvoiceDuplicateGet)

		// Recurring invoice management
		r.Get("/recurring", h.RecurringList)
		r.Post("/recurring/{id}/pause", h.RecurringPause)
		r.Post("/recurring/{id}/resume", h.RecurringResume)
		r.Post("/recurring/{id}/delete", h.RecurringDelete)
	})
	return r
}
