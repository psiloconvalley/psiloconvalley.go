// internal/handlers/estimate_send.go
package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	
	"psiloconvalley/internal/audit"
	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/mailer"
	"psiloconvalley/internal/views"
)

// EstimateSendGet renders the send form for an estimate.
// Protected — requires login.
func (h *Handlers) EstimateSendGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil || !h.canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	if inv.DocumentType != "estimate" {
		http.NotFound(w, r)
		return
	}

	invoiceView := views.MapInvoicePage(inv, items, "view")

	h.App.Render(w, r, "invoice_send.tmpl", map[string]any{
		"Invoice":      invoiceView,
		"ClientEmail":  inv.ClientEmail,
		"InvoiceID":    id,
		"IsEstimate":   true,
		"DocumentType": "estimate",
	})
}

// EstimateSendPost sends the estimate email to the client and advances
// status from draft → sent.
// Protected — requires login.
func (h *Handlers) EstimateSendPost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil || !h.canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	if inv.DocumentType != "estimate" {
		http.NotFound(w, r)
		return
	}

	toEmail := r.FormValue("to_email")
	if toEmail == "" {
		http.Error(w, "Recipient email is required", http.StatusBadRequest)
		return
	}

	token, err := h.App.InvRepo.EnsurePublicToken(r.Context(), id)
	if err != nil {
		log.Printf("[estimate] failed to ensure public token for estimate %d: %v", id, err)
	}

	respondURL := fmt.Sprintf("%s/estimates/%d/respond", h.App.BaseURL, id)
	if token != "" {
		respondURL += "?access=" + token
	}

	invoiceView := views.MapInvoicePage(inv, items, "view")

	validUntil := ""
	if inv.DueDate != nil {
		validUntil = inv.DueDate.Format("January 2, 2006")
	}

	emailData := mailer.EstimateEmailData{
		EstimateNumber: inv.InvoiceNumber,
		ClientName:     inv.ClientName,
		CompanyName:    inv.CompanyName,
		Total:          invoiceView.Total,
		Currency:       inv.Currency,
		ValidUntil:     validUntil,
		RespondURL:     respondURL,
		PersonalNote:   r.FormValue("personal_note"),
	}

	if err := h.App.Mailer.SendEstimate(toEmail, emailData); err != nil {
		log.Printf("[estimate] email send error: %v", err)
		http.Error(w, "Failed to send estimate email. Please try again.", http.StatusInternalServerError)
		return
	}

	// Advance status draft → sent via repo — no raw SQL in handlers.
	if inv.Status == "draft" {
		if err := h.App.InvRepo.UpdateEstimateStatus(r.Context(), id, user.ID, "sent"); err != nil {
			log.Printf("[estimate] failed to advance status to sent for estimate %d: %v", id, err)
		}
	}

	log.Printf("[estimate] estimate %s sent to %s by user %d", inv.InvoiceNumber, toEmail, user.ID)
	audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
		UserID:     audit.UserIDPtr(user.ID),
		Action:     audit.ActionEstimateSent,
		EntityType: audit.EntityEstimate,
		EntityID:   audit.EntityIDPtr(id),
		IPAddress:  audit.IPFromRequest(r),
		Metadata:   map[string]any{"to_email": toEmail},
	})
	http.Redirect(w, r,
		fmt.Sprintf("/estimates/%d?sent=true&to=%s", id, toEmail),
		http.StatusSeeOther,
	)
}

