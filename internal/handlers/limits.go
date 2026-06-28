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
	freeInvoiceLimit  = 5
	freeSendLimit     = 3
	freeClientLimit   = 5
	freeEstimateLimit = 3
	freeReportLimit   = 0

	growthInvoiceLimit  = 15
	growthSendLimit     = 15
	growthClientLimit   = 15
	growthEstimateLimit = 15
	growthReportLimit   = 1
)

// ── Plan helpers ─────────────────────────────────────────────────────

func isPro(plan string) bool    { return plan == "pro" }
func isGrowth(plan string) bool { return plan == "growth" }
func isPaid(plan string) bool   { return plan == "growth" || plan == "pro" }

func clientLimitFor(plan string) int {
	if isPro(plan) {
		return 0
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

// ── Core metering helper ─────────────────────────────────────────────
//
// usageAllowed is the single source of truth for monthly quota checks.
//
// Failure posture: fail-open on meter read errors.
// Quota checks are commercial controls, not security boundaries.
// A broken meter must never block a legitimate paying user.
//
// Returns true if the action is allowed.
func (h *Handlers) usageAllowed(r *http.Request, feature string, freeLimit, growthLimit int) bool {
	user := auth.GetUser(r)
	if user == nil {
		return false
	}
	if isPro(user.Plan) {
		return true
	}

	count, err := h.App.UsageRepo.Get(r.Context(), user.ID, feature)
	if err != nil {
		// Fail open — meter read error must never block a legitimate user.
		// Quota checks are commercial controls, not security boundaries.
		// The warning log ensures the underlying issue is visible in Railway.
		slog.Warn("usage meter read failed, allowing user",
			"feature", feature,
			"user_id", user.ID,
			"err", err,
		)
		return true
	}

	limit := freeLimit
	if isGrowth(user.Plan) {
		limit = growthLimit
	}
	return count < limit
}

// ── Client limit ─────────────────────────────────────────────────────
// Uses ClientRepo.CountByUserID (not UsageRepo) — total count, not monthly.
// Same fail-open posture as usageAllowed.

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
		slog.Warn("client count read failed, allowing user",
			"user_id", user.ID,
			"err", err,
		)
		return true
	}

	limit := freeClientLimit
	if isGrowth(user.Plan) {
		limit = growthClientLimit
	}
	return count < limit
}

// ── Metered feature gates ────────────────────────────────────────────
// Each function is a thin wrapper around usageAllowed.
// Policy lives in usageAllowed — not here.

// hasReachedLimit returns true when the user cannot create another invoice.
// Inverted style kept for backwards compatibility with existing callers.
func (h *Handlers) hasReachedLimit(r *http.Request) bool {
	user := auth.GetUser(r)
	if user == nil {
		return auth.AnonLimitReached(r)
	}
	return !h.usageAllowed(r, "invoices", freeInvoiceLimit, growthInvoiceLimit)
}

func (h *Handlers) canSendInvoice(r *http.Request) bool {
	return h.usageAllowed(r, "sends", freeSendLimit, growthSendLimit)
}

func (h *Handlers) canCreateEstimate(r *http.Request) bool {
	return h.usageAllowed(r, "estimates", freeEstimateLimit, growthEstimateLimit)
}

func (h *Handlers) canViewReport(r *http.Request) bool {
	return h.usageAllowed(r, "reports", freeReportLimit, growthReportLimit)
}

// ── Feature gates (boolean, no counting) ─────────────────────────────
// These are entitlement checks, not quota checks.
// Fail-closed is correct here — plan must be confirmed.

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

// ── Invoice access control ────────────────────────────────────────────
// These are security/authorization boundaries.
// Fail-closed is correct here — uncertain access must be denied.

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
