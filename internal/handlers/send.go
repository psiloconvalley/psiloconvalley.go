package handlers

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
	"net/url"

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
	if user.Plan != "pro" {
		http.Redirect(w, r, "/pricing?reason=email-pro", http.StatusSeeOther)
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
	if user.Plan != "pro" {
		http.Redirect(w, r, "/pricing?reason=email-pro", http.StatusSeeOther)
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

	if err := h.App.Templates.ExecuteTemplate(&buf, invoiceTemplateName(invoiceView.TemplateID), templateData); err != nil {
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

	// ── Schedule automatic reminders if enabled ──────────────────────
	// Cancel any existing pending reminders first to prevent duplicates
	// when the same invoice is sent more than once.
	if inv.AutoReminders && inv.DueDate != nil {
		cancelled, err := h.App.SchedulerRepo.CancelJobsForInvoice(r.Context(), inv.ID)
		if err != nil {
			log.Printf("[send] warning: failed to cancel existing reminders for invoice %d: %v", inv.ID, err)
		} else if cancelled > 0 {
			log.Printf("[send] cleared %d stale reminder jobs for invoice %d before rescheduling", cancelled, inv.ID)
		}
		h.scheduleReminders(r.Context(), inv.ID, *inv.DueDate)
	}

http.Redirect(w, r,
	fmt.Sprintf("/invoices/%d?sent=true&to=%s", id, url.QueryEscape(toEmail)),
	http.StatusSeeOther,
)
}


// scheduleReminders queues reminder jobs based on the invoice due date.
// Jobs are inserted into scheduled_jobs and the engine picks them up
// automatically when each run_at arrives.
func (h *Handlers) scheduleReminders(ctx context.Context, invoiceID int64, dueDate time.Time) {
	type reminder struct {
		offset       time.Duration
		reminderType string
	}

	schedule := []reminder{
		{-3 * 24 * time.Hour, "due_soon"},     // 3 days before
		{0, "due_today"},                        // day of
		{3 * 24 * time.Hour, "overdue"},         // 3 days after
		{7 * 24 * time.Hour, "overdue"},         // 7 days after
		{14 * 24 * time.Hour, "overdue"},        // 14 days after
	}

	for _, r := range schedule {
		runAt := dueDate.Add(r.offset)

		// Don't schedule reminders in the past
		if runAt.Before(time.Now()) {
			continue
		}

		payload := map[string]any{
			"invoice_id":    invoiceID,
			"reminder_type": r.reminderType,
		}

		jobID, err := h.App.SchedulerRepo.CreateJob(ctx, "send_reminder", payload, runAt)
		if err != nil {
			log.Printf("[send] failed to schedule %s reminder for invoice %d: %v",
				r.reminderType, invoiceID, err)
			continue
		}

		log.Printf("[send] scheduled %s reminder for invoice %d (job %d, run_at %s)",
			r.reminderType, invoiceID, jobID, runAt.Format("2006-01-02 15:04"))
	}
}
