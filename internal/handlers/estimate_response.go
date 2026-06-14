// internal/handlers/estimate_response.go
package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"psiloconvalley/internal/mailer"
	"psiloconvalley/internal/repo"
	"psiloconvalley/internal/views"
)

// EstimateRespondGet shows the client-facing response page.
// Public route — no login required. Access controlled by public_token.
func (h *Handlers) EstimateRespondGet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil {
		http.NotFound(w, r)
		return
	}

	if inv.DocumentType != "estimate" {
		http.NotFound(w, r)
		return
	}

	accessToken := r.URL.Query().Get("access")
	if inv.PublicToken == "" || accessToken == "" || accessToken != inv.PublicToken {
		http.Error(w, "Invalid or missing access token", http.StatusForbidden)
		return
	}

	if inv.Status == "accepted" || inv.Status == "rejected" || inv.Status == "converted" {
		h.App.Render(w, r, "estimate_responded.tmpl", map[string]any{
			"Status":      inv.Status,
			"EstimateNum": inv.InvoiceNumber,
			"CompanyName": inv.CompanyName,
		})
		return
	}

	invoiceView := views.MapInvoicePage(inv, items, "view")

	h.App.Render(w, r, "estimate_respond.tmpl", map[string]any{
		"Invoice":     invoiceView,
		"AccessToken": accessToken,
		"IsEstimate":  true,
	})
}

// EstimateRespondPost handles the client's accept/reject/suggest response.
// Public route — no login required. Access controlled by public_token.
func (h *Handlers) EstimateRespondPost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inv, _, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil {
		http.NotFound(w, r)
		return
	}

	if inv.DocumentType != "estimate" {
		http.NotFound(w, r)
		return
	}

	accessToken := r.URL.Query().Get("access")
	if inv.PublicToken == "" || accessToken == "" || accessToken != inv.PublicToken {
		http.Error(w, "Invalid or missing access token", http.StatusForbidden)
		return
	}

	if inv.Status != "sent" {
		http.Redirect(w, r,
			fmt.Sprintf("/estimates/%d/respond?access=%s", id, accessToken),
			http.StatusSeeOther,
		)
		return
	}

	action := r.FormValue("action")
	message := strings.TrimSpace(r.FormValue("message"))
	clientName := strings.TrimSpace(r.FormValue("client_name"))

	validActions := map[string]bool{
		"accepted": true, "rejected": true, "suggestion": true,
	}
	if !validActions[action] {
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	resp := &repo.EstimateResponse{
		EstimateID: id,
		Action:     action,
		Message:    message,
		ClientName: clientName,
	}
	if err := h.App.EstRespRepo.Create(r.Context(), resp); err != nil {
		log.Printf("[estimate] response save error: %v", err)
	}

	// Update status for accept/reject via repo — no raw SQL in handlers.
	if action == "accepted" || action == "rejected" {
		if inv.UserID != nil {
			if err := h.App.InvRepo.UpdateEstimateStatus(r.Context(), id, *inv.UserID, action); err != nil {
				log.Printf("[estimate] failed to update status for estimate %d: %v", id, err)
			}
		}
	}

	if inv.UserID != nil {
		owner, err := h.App.UserRepo.GetByID(*inv.UserID)
		if err == nil && owner.Email != "" {
			estimateURL := fmt.Sprintf("%s/estimates/%d", h.App.BaseURL, id)
			_ = h.App.Mailer.SendEstimateResponse(owner.Email, mailer.EstimateResponseEmailData{
				EstimateNumber: inv.InvoiceNumber,
				ClientName:     clientName,
				CompanyName:    inv.CompanyName,
				Action:         action,
				Message:        message,
				EstimateURL:    estimateURL,
			})
		}
	}

	log.Printf("[estimate] client responded to estimate %d: action=%s", id, action)

	h.App.Render(w, r, "estimate_responded.tmpl", map[string]any{
		"Action":       action,
		"EstimateNum":  inv.InvoiceNumber,
		"CompanyName":  inv.CompanyName,
		"IsSuggestion": action == "suggestion",
	})
}
