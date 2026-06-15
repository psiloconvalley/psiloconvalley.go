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
	r.NotFound(h.NotFound)


	r.Use(handlers.SecurityHeaders)
	r.Use(auth.LoadUser(h.App.UserRepo))

	r.Handle("/static/*", http.StripPrefix("/static/",
		http.FileServer(http.Dir("static"))))

	// ── Public routes (no login required) ──────────────────────────
	r.Get("/", h.Index)
	r.Get("/healthz", h.Health)
	r.Get("/research", h.Research)
	r.Get("/security", h.SecurityPage)
	r.Get("/tools", h.ToolsHub)
	r.Get("/pricing", h.PricingGet)
	r.Post("/stripe/webhook", h.StripeWebhook)
	r.Post("/feedback", h.Feedback)

	r.Get("/register", h.RegisterGet)
	r.With(auth.RateLimitRegister).Post("/register", h.RegisterPost)

	r.Get("/login", h.LoginGet)
	r.With(auth.RateLimitLogin).Post("/login", h.LoginPost)

	r.Post("/logout", h.Logout)

	r.Get("/auth/google", auth.GoogleLoginHandler)
	r.Get("/auth/google/callback", h.GoogleCallback)
	r.Get("/forgot-password", h.ForgotPasswordGet)
	r.With(auth.RateLimitForgotPassword).Post("/forgot-password", h.ForgotPasswordPost)
	r.Get("/auth/magic", h.MagicLinkGet)
	r.With(auth.RateLimitMagicLink).Post("/auth/magic", h.MagicLinkPost)

	// ── Freemium invoice routes (ownership enforced in handlers) ───
	r.Get("/invoices/new", h.InvoiceNewGet)
	r.Post("/invoices/create", h.InvoiceCreatePost)
	r.Get("/invoices/{id}", h.InvoiceDetail)
	r.Get("/invoices/{id}/pdf", h.InvoicePDFGet)
	r.Get("/invoices/{id}/pay", h.InvoicePayGet)


	// ── Public estimate response routes ───────────────────────────
	r.Get("/estimates/{id}/respond", h.EstimateRespondGet)
	r.Post("/estimates/{id}/respond", h.EstimateRespondPost)
	// ── Protected routes (login required) ──────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/checkout", h.CheckoutPost)
		r.Post("/billing/portal", h.BillingPortalPost)
		r.Get("/auth/stripe/connect", h.StripeConnectStart)
		r.Get("/auth/stripe/callback", h.StripeConnectCallback)

		r.Get("/profile", h.ProfileGet)
		r.Post("/profile", h.ProfilePost)
		r.Post("/profile/password", h.ChangePasswordPost)

		r.Get("/clients", h.ClientsList)
		r.Get("/clients/new", h.ClientNewGet)
		r.Post("/clients/new", h.ClientNewPost)
		r.Get("/clients/{id}/edit", h.ClientEditGet)
		r.Post("/clients/{id}/edit", h.ClientEditPost)
		r.Post("/clients/{id}/delete", h.ClientDelete)

		// Invoice management
		r.Get("/dashboard", h.DashboardGet)
		r.Get("/invoices", h.InvoicesList)
		r.Get("/invoices/{id}/edit", h.InvoiceEditGet)
		r.Post("/invoices/{id}/edit", h.InvoiceUpdatePost)
		r.Post("/invoices/{id}/status", h.InvoiceStatusPost)
		r.Post("/invoices/{id}/delete", h.InvoiceDeletePost)
		r.Get("/invoices/{id}/send", h.InvoiceSendGet)
		r.Post("/invoices/{id}/send", h.InvoiceSendPost)
		r.Get("/invoices/{id}/duplicate", h.InvoiceDuplicateGet)

		// Recurring invoice management
		r.Get("/recurring", h.RecurringList)
		r.Post("/recurring/{id}/pause", h.RecurringPause)
		r.Post("/recurring/{id}/resume", h.RecurringResume)
		r.Post("/recurring/{id}/delete", h.RecurringDelete)

		// Reports
		r.Get("/reports", h.ReportGet)
		r.Get("/reports/export.csv", h.ReportExportCSV)
		r.Get("/reports/clients", h.ClientScorecardGet)
		// Expense tracking
		r.Get("/expenses", h.ExpensesList)
		r.Get("/expenses/new", h.ExpenseNewGet)
		r.Post("/expenses/create", h.ExpenseCreatePost)
		r.Get("/expenses/{id}/edit", h.ExpenseEditGet)
		r.Post("/expenses/{id}/edit", h.ExpenseEditPost)
		r.Post("/expenses/{id}/delete", h.ExpenseDeletePost)

		// Estimate management
		r.Get("/estimates", h.EstimatesList)
		r.Get("/estimates/new", h.EstimateNewGet)
		r.Post("/estimates/create", h.EstimateCreatePost)
		r.Get("/estimates/{id}", h.EstimateDetail)
		r.Get("/estimates/{id}/edit", h.EstimateEditGet)
		r.Post("/estimates/{id}/edit", h.EstimateEditPost)
		r.Post("/estimates/{id}/status", h.EstimateStatusPost)
		r.Post("/estimates/{id}/delete", h.EstimateDeletePost)
		r.Post("/estimates/{id}/convert", h.EstimateConvertPost)
		r.Get("/estimates/{id}/send", h.EstimateSendGet)
		r.Post("/estimates/{id}/send", h.EstimateSendPost)

		// Admin
		r.Get("/admin/analytics", h.AdminAnalytics)
		r.Get("/admin/audit", h.AdminAuditLog)
	})
	return r
}
