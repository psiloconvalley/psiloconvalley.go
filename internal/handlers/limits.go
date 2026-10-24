// internal/handlers/limits.go
// Plan limit enforcement — the single source of truth for quota checks.
// All plan constants and helpers live in internal/catalog/plans.go.
// This file only contains the handler-level enforcement logic.
package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/repo"
)

// ── Core metering helper ─────────────────────────────────────────────

// usageStore is the subset of repo.UsageRepo this package depends on.
// Defined here (not in repo/) so handlers own the contract — Go stdlib
// convention of "accept interfaces, return structs."
// *repo.UsageRepo satisfies this in production; *fakeUsageStore in tests.
type usageStore interface {
	Get(ctx context.Context, userID int64, column string) (int, error)
	Increment(ctx context.Context, userID int64, column string) error
}

// usageAllowedFor is the pure business logic — no HTTP, no handler state.
// All inputs explicit, all outputs returned. This is what tests exercise.
//
// Failure posture: fail-OPEN on meter read errors.
// Quota checks are commercial controls, not security boundaries.
func usageAllowedFor(ctx context.Context, store usageStore, user *repo.User, feature string, freeLimit, proLimit int) bool {
	if user == nil {
		return false
	}
	if catalog.IsUnlimited(user.ID, user.Plan) {
		return true
	}

	count, err := store.Get(ctx, user.ID, feature)
	if err != nil {
		slog.Warn("usage meter read failed, allowing user",
			"feature", feature,
			"user_id", user.ID,
			"err", err,
		)
		return true // fail open
	}

	limit := freeLimit
	if catalog.IsPro(user.Plan) {
		limit = proLimit
	}
	if limit == 0 {
		return true // 0 = unlimited for this feature at this tier
	}
	return count < limit
}

// usageAllowed is the thin HTTP wrapper. Pulls user + context out of the
// request and delegates to usageAllowedFor.
func (h *Handlers) usageAllowed(r *http.Request, feature string, freeLimit, proLimit int) bool {
	return usageAllowedFor(
		r.Context(),
		h.App.UsageRepo,
		auth.GetUser(r),
		feature,
		freeLimit,
		proLimit,
	)
}
// ── Client limit ─────────────────────────────────────────────────────
func (h *Handlers) canAddClient(r *http.Request) bool {
	user := auth.GetUser(r)
	if user == nil {
		return false
	}
	if catalog.IsUnlimited(user.ID, user.Plan) {
		return true
	}

	count, err := h.App.ClientRepo.CountByUserID(r.Context(), user.ID)
	if err != nil {
		slog.Warn("client count read failed, allowing user",
			"user_id", user.ID, "err", err)
		return true
	}

	limit := catalog.FreeClientLimit
	if catalog.IsPro(user.Plan) {
		limit = catalog.ProClientLimit
	}
	return count < limit
}

// ── Metered feature gates ────────────────────────────────────────────

func (h *Handlers) hasReachedLimit(r *http.Request) bool {
	user := auth.GetUser(r)
	if user == nil {
		return auth.AnonLimitReached(r)
	}
	return !h.usageAllowed(r, "invoices", catalog.FreeInvoiceLimit, catalog.ProInvoiceLimit)
}

func (h *Handlers) canSendInvoice(r *http.Request) bool {
	return h.usageAllowed(r, "sends", catalog.FreeSendLimit, catalog.ProSendLimit)
}

func (h *Handlers) canCreateEstimate(r *http.Request) bool {
	return h.usageAllowed(r, "estimates", catalog.FreeEstimateLimit, catalog.ProEstimateLimit)
}

func (h *Handlers) canViewReport(r *http.Request) bool {
	return h.usageAllowed(r, "reports", catalog.FreeReportLimit, catalog.ProReportLimit)
}

// ── Feature gates (boolean, no counting) ─────────────────────────────

func (h *Handlers) canAccessExpenses(r *http.Request) bool {
	user := auth.GetUser(r)
	if user == nil {
		return false
	}
	return catalog.IsPaid(user.Plan)
}

func (h *Handlers) canAccessRecurring(r *http.Request) bool {
	user := auth.GetUser(r)
	if user == nil {
		return false
	}
	return catalog.HasAutomation(user.ID, user.Plan)
}

func (h *Handlers) canAccessStripePayments(r *http.Request) bool {
	user := auth.GetUser(r)
	if user == nil {
		return false
	}
	return catalog.IsPaid(user.Plan)
}

// ── Invoice access control ────────────────────────────────────────────

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
