package handlers

import (
	"net/http"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/repo"
)

const freePlanMonthlyLimit = 100

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
		return true
	}

	return count < 3
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
		return false
	}

	return count >= freePlanMonthlyLimit
}

func (h *Handlers) canAccessInvoice(r *http.Request, inv *repo.Invoice) bool {
	if inv.UserID == nil {
		return true
	}

	user := auth.GetUser(r)
	if user == nil {
		return false
	}

	return user.ID == *inv.UserID
}
