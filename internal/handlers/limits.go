// internal/handlers/limits.go
package handlers

import (
	"log/slog"
	"net/http"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/repo"
)

// ── Plan limits ──────────────────────────────────────────────────────
// All business rules in one place. A product decision to change any
// tier limit is a one-line diff.
//
// Locked pricing matrix (June 2025):
//   Feature        Free    Growth($8.88)   Pro($18.88)
//   Invoices       5/mo    15/mo           Unlimited
//   Sends          3/mo    15/mo           Unlimited
//   Clients        5       15              Unlimited
//   Estimates      3/mo    15/mo           Unlimited
//   Reports        0       1/mo            Unlimited
//   Expenses       ✗       ✓               ✓
//   Recurring      ✗       ✗               ✓
//   Reminders      ✗       ✗               ✓
//   Stripe pay     ✗       ✓               ✓
//   Adv templates  ✗       ✗               ✓

const (
	freeInvoiceLimit   = 5
	freeSendLimit      = 3
	freeClientLimit    = 5
	freeEstimateLimit  = 3
	freeReportLimit    = 0

	growthInvoiceLimit  = 15
	growthSendLimit     = 15
	growthClientLimit   = 15
	growthEstimateLimit = 15
	growthReportLimit   = 1
)

// ── Plan helper ──────────────────────────────────────────────────────

func isPro(plan string) bool    { return plan == "pro" }
func isGrowth(plan string) bool { return plan == "growth" }
func isPaid(plan string) bool   { return plan == "growth" || plan == "pro" }
func clientLimitFor(plan string) int {
	if isPro(plan) {
		return 0 // unlimited — caller should check isPro first
	}
	if isGrowth(plan) {
		return growthClientLimit
	}
	return freeClientLimit
}

func invoiceLimitFor(plan string) int {
	if isPro(plan) {
		return 0
	}
	if isGrowth(plan) {
		return growthInvoiceLimit
	}
	return freeInvoiceLimit
}

// ── Client limit ─────────────────────────────────────────────────────

func (h *Handlers) canAddClient(r *http.Request) bool {
	user := auth.GetUser(r)
	if user == nil {
		return false
	}
	if isPro(user.Plan) {
		return true
	}

	count, err := h.App.ClientRepo.CountByUserID(r.Context(), user.ID)
	if err != nil {
		slog.Error("client limit check failed", "user_id", user.ID, "err", err)
		return false
	}

	limit := freeClientLimit
	if isGrowth(user.Plan) {
		limit = growthClientLimit
	}
	return count < limit
}

// ── Invoice creation limit ───────────────────────────────────────────

func (h *Handlers) hasReachedLimit(r *http.Request) bool {
	user := auth.GetUser(r)
	if user == nil {
		return auth.AnonLimitReached(r)
	}
	if isPro(user.Plan) {
		return false
	}

	count, err := h.App.UsageRepo.Get(r.Context(), user.ID, "invoices")
	if err != nil {
	slog.Warn("invoice limit check failed, allowing user", "user_id", user.ID, "err", err)
	return false
}
	limit := freeInvoiceLimit
	if isGrowth(user.Plan) {
		limit = growthInvoiceLimit
	}
	return count >= limit
}

// ── Send limit ───────────────────────────────────────────────────────

func (h *Handlers) canSendInvoice(r *http.Request) bool {
	user := auth.GetUser(r)
	if user == nil {
		return false
	}
	if isPro(user.Plan) {
		return true
	}

	count, err := h.App.UsageRepo.Get(r.Context(), user.ID, "sends")
	if err != nil {
		slog.Error("send limit check failed", "user_id", user.ID, "err", err)
		return false
	}

	limit := freeSendLimit
	if isGrowth(user.Plan) {
		limit = growthSendLimit
	}
	return count < limit
}

// ── Estimate limit ───────────────────────────────────────────────────

func (h *Handlers) canCreateEstimate(r *http.Request) bool {
	user := auth.GetUser(r)
	if user == nil {
		return false
	}
	if isPro(user.Plan) {
		return true
	}

	count, err := h.App.UsageRepo.Get(r.Context(), user.ID, "estimates")
	if err != nil {
		slog.Error("estimate limit check failed", "user_id", user.ID, "err", err)
		return false
	}

	limit := freeEstimateLimit
	if isGrowth(user.Plan) {
		limit = growthEstimateLimit
	}
	return count < limit
}

// ── Report limit ─────────────────────────────────────────────────────

func (h *Handlers) canViewReport(r *http.Request) bool {
	user := auth.GetUser(r)
	if user == nil {
		return false
	}
	if isPro(user.Plan) {
		return true
	}

	count, err := h.App.UsageRepo.Get(r.Context(), user.ID, "reports")
	if err != nil {
		slog.Error("report limit check failed", "user_id", user.ID, "err", err)
		return false
	}

	limit := freeReportLimit
	if isGrowth(user.Plan) {
		limit = growthReportLimit
	}
	return count < limit
}

// ── Feature gates (boolean, no counting) ─────────────────────────────

func (h *Handlers) canAccessExpenses(r *http.Request) bool {
	user := auth.GetUser(r)
	if user == nil {
		return false
	}
	return isPaid(user.Plan)
}

func (h *Handlers) canAccessRecurring(r *http.Request) bool {
	user := auth.GetUser(r)
	if user == nil {
		return false
	}
	return isPro(user.Plan)
}

func (h *Handlers) canAccessStripePayments(r *http.Request) bool {
	user := auth.GetUser(r)
	if user == nil {
		return false
	}
	return isPaid(user.Plan)
}

// ── Invoice access control (unchanged) ───────────────────────────────

func (h *Handlers) canAccessInvoice(r *http.Request, inv *repo.Invoice) bool {
	if inv == nil {
		return false
	}

	user := auth.GetUser(r)

	if inv.UserID != nil {
		if user == nil {
			return false
		}
		return user.ID == *inv.UserID
	}

	if user != nil {
		return false
	}

	anonToken, ok := auth.GetAnonymousToken(r)
	if !ok || anonToken == "" || inv.AnonymousToken == "" {
		return false
	}

	return anonToken == inv.AnonymousToken
}

func (h *Handlers) canViewInvoice(r *http.Request, inv *repo.Invoice) bool {
	if inv == nil {
		return false
	}

	user := auth.GetUser(r)
	if inv.UserID != nil && user != nil && user.ID == *inv.UserID {
		return true
	}

	if inv.AnonymousToken != "" {
		anonToken, ok := auth.GetAnonymousToken(r)
		if ok && anonToken == inv.AnonymousToken {
			return true
		}
	}

	if inv.PublicToken != "" {
		accessToken := r.URL.Query().Get("access")
		if accessToken != "" && accessToken == inv.PublicToken {
			return true
		}
	}

	return false
}
