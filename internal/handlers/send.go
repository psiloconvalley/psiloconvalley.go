package handlers

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

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
		"Invoice":     invoiceView,
		"ClientEmail": inv.ClientEmail,
		"InvoiceID":   id,
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

	toEmail := r.FormValue("to_email")
	personalNote := r.FormValue("personal_note")

	if toEmail == "" {
		http.Error(w, "Recipient email is required", http.StatusBadRequest)
		return
	}

	// ── Render invoice HTML for PDF ───────────────────────────────────
	invoiceView := views.MapInvoicePage(inv, items, "view")

	var buf bytes.Buffer
	templateData := map[string]any{
		"Invoice":   invoiceView,
		"User":      user,
		"csrfField": "",
	}

	if err := h.App.Templates.ExecuteTemplate(&buf, "invoice_detail.tmpl", templateData); err != nil {
		log.Printf("[send] template render error: %v", err)
		http.Error(w, "Could not render invoice", http.StatusInternalServerError)
		return
	}

	// ── Generate PDF with a fresh context (not r.Context()) ──────────
	// r.Context() can be cancelled by the browser before Chromium
	// finishes. A detached context with its own timeout fixes this.
	pdfCtx, pdfCancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer pdfCancel()

	pdfBytes, err := pdf.Generate(pdfCtx, buf.String())
	if err != nil {
		log.Printf("[send] pdf generation error: %v", err)
		http.Error(w, "Could not generate PDF", http.StatusInternalServerError)
		return
	}

	// ── Build email payload ───────────────────────────────────────────
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

	// ── Send via Resend ───────────────────────────────────────────────
	pdfFilename := fmt.Sprintf("invoice-%s.pdf", inv.InvoiceNumber)

	if err := h.App.Mailer.SendInvoice(toEmail, emailData, pdfBytes, pdfFilename); err != nil {
		log.Printf("[send] email send error: %v", err)
		http.Error(w, "Failed to send email. Please try again.", http.StatusInternalServerError)
		return
	}

	// ── Auto-advance status draft → sent ─────────────────────────────
	if inv.Status == "draft" && inv.UserID != nil {
		if err := h.App.InvRepo.UpdateInvoiceStatus(
			r.Context(), id, "sent", *inv.UserID,
		); err != nil {
			// Non-fatal — email already sent successfully
			log.Printf("[send] status update warning: %v", err)
		}
	}

	log.Printf("[send] invoice %s sent to %s by user %d",
		inv.InvoiceNumber, toEmail, user.ID)

	http.Redirect(w, r,
		fmt.Sprintf("/invoices/%d?sent=true", id),
		http.StatusSeeOther,
	)
}
