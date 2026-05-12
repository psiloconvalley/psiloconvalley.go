package handlers

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/mailer"
	"psiloconvalley/internal/pdf"
	"psiloconvalley/internal/views"
)

// =====================================================================
// InvoiceSendGet — show the send form
// =====================================================================

func (h *Handlers) InvoiceSendGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || !h.canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	invoiceView := views.MapInvoicePage(inv, items, "view")

	h.App.Render(w, r, "invoice_send.tmpl", map[string]any{
		"Invoice":       invoiceView,
		"ClientEmail":   inv.ClientEmail,
		"InvoiceID":     id,
	})
}

// =====================================================================
// InvoiceSendPost — generate PDF and send email
// =====================================================================

func (h *Handlers) InvoiceSendPost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || !h.canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	// Get form values
	toEmail := r.FormValue("to_email")
	personalNote := r.FormValue("personal_note")

	if toEmail == "" {
		http.Error(w, "Recipient email is required", http.StatusBadRequest)
		return
	}

	// ── Generate PDF ──────────────────────────────────────────────────
	invoiceView := views.MapInvoicePage(inv, items, "view")

	var buf bytes.Buffer
	data := map[string]any{
		"Invoice":   invoiceView,
		"User":      user,
		"csrfField": "",
	}

	if err := h.App.Templates.ExecuteTemplate(&buf, "invoice_detail.tmpl", data); err != nil {
		log.Printf("[send] template render error: %v", err)
		http.Error(w, "Could not render invoice", http.StatusInternalServerError)
		return
	}

	pdfBytes, err := pdf.Generate(r.Context(), buf.String())
	if err != nil {
		log.Printf("[send] pdf generation error: %v", err)
		http.Error(w, "Could not generate PDF", http.StatusInternalServerError)
		return
	}

	// ── Build email data ──────────────────────────────────────────────
	baseURL := h.App.BaseURL
	invoiceURL := fmt.Sprintf("%s/invoices/%d", baseURL, id)

	emailData := mailer.InvoiceEmailData{
		InvoiceNumber: inv.InvoiceNumber,
		ClientName:    inv.ClientName,
		CompanyName:   inv.CompanyName,
		Total:         invoiceView.Total,
		Currency:      inv.Currency,
		InvoiceURL:    invoiceURL,
		PersonalNote:  personalNote,
	}

	if inv.DueDate != nil {
		emailData.DueDate = inv.DueDate.Format("January 2, 2006")
	}

	// ── Send email ────────────────────────────────────────────────────
	pdfFilename := fmt.Sprintf("invoice-%s.pdf", inv.InvoiceNumber)

	if err := h.App.Mailer.SendInvoice(toEmail, emailData, pdfBytes, pdfFilename); err != nil {
		log.Printf("[send] email send error: %v", err)
		http.Error(w, "Failed to send email. Please try again.", http.StatusInternalServerError)
		return
	}

	// ── Auto-update status draft → sent ──────────────────────────────
	if inv.Status == "draft" && inv.UserID != nil {
		if err := h.App.InvRepo.UpdateInvoiceStatus(
			r.Context(), id, "sent", *inv.UserID,
		); err != nil {
			log.Printf("[send] status update warning: %v", err)
			// Non-fatal — email already sent
		}
	}

	log.Printf("[send] invoice %s sent to %s by user %d",
		inv.InvoiceNumber, toEmail, user.ID)

	http.Redirect(w, r,
		fmt.Sprintf("/invoices/%d?sent=true", id),
		http.StatusSeeOther,
	)
}
