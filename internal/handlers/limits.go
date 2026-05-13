// internal/handlers/limits.go
package handlers

import (
	"log"
	"net/http"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/repo"
)

// Plan limits — all business rules in one place.
// FIX D2: freePlanClientLimit is now a named constant, consistent with
// freePlanMonthlyLimit. Magic numbers in access control are dangerous —
// a product decision to change the free tier limit should be a one-line
// diff, not a grep-and-replace across the codebase.
const (
	freePlanMonthlyLimit = 100
	freePlanClientLimit  = 3
)

func (h *Handlers) canAddClient(r *http.Request) bool {
	user := auth.GetUser(r)
	if user == nil {
		return false
	}
	if user.Plan == "pro" {
		return true
	}

	count, err := h.App.ClientRepo.CountByUserID(r.Context(), user.ID)
	if err != nil {
		// FIX D3 (partial): DB error on client count — fail closed.
		// We cannot verify the limit, so we do not allow the addition.
		// Log it so the on-call engineer can see DB connectivity issues.
		log.Printf("canAddClient: db error for user %d: %v", user.ID, err)
		return false
	}

	return count < freePlanClientLimit // FIX D2: named constant
}

func (h *Handlers) hasReachedLimit(r *http.Request) bool {
	user := auth.GetUser(r)

	if user == nil {
		return auth.AnonLimitReached(r)
	}

	if user.Plan == "pro" {
		return false
	}

	count, err := h.App.UserRepo.GetMonthlyInvoiceCount(r.Context(), user.ID)
	if err != nil {
		// FIX D3: DB error — fail closed (limit IS reached).
		// Previously this returned false (limit not reached), meaning a
		// DB outage granted unlimited invoice creation to all free users.
		// Failing closed is the safer default for a metered resource.
		log.Printf("hasReachedLimit: db error for user %d: %v", user.ID, err)
		return true
	}

	return count >= freePlanMonthlyLimit
}

// canAccessInvoice is the single authorisation check for all invoice
// operations. An invoice with no UserID is a guest invoice — accessible
// to anyone (anonymous creation flow). An invoice with a UserID requires
// the requesting user to match.
//
// FIX D1: The exported duplicate CanAccessInvoice in middleware.go has
// been removed. This method is the sole implementation.
func (h *Handlers) canAccessInvoice(r *http.Request, inv *repo.Invoice) bool {
	if inv.UserID == nil {
		return true // guest invoice
	}
	user := auth.GetUser(r)
	if user == nil {
		return false
	}
	return user.ID == *inv.UserID
}
