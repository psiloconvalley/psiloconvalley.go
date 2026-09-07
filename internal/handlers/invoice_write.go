// internal/handlers/invoice_write.go
package handlers

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"psiloconvalley/internal/audit"
	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/mailer"
	"psiloconvalley/internal/repo"
	"psiloconvalley/internal/service"
)

// InvoiceUpdatePost handles updates to an existing invoice.
func (h *Handlers) InvoiceUpdatePost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inv, _, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil || !h.canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	// ── Parse line items via service ──────────────────────────────────
	items := service.ParseLineItems(service.LineItemInput{
		Descriptions: r.Form["description[]"],
		Details:      r.Form["details[]"],
		Quantities:   r.Form["quantity[]"],
		UnitPrices:   r.Form["unit_price[]"],
	})

	// ── Parse dates ───────────────────────────────────────────────────
	if d := r.FormValue("issue_date"); d != "" {
		if t, err := time.Parse("2006-01-02", d); err == nil {
			inv.IssueDate = t
		}
	}
	var dueDate *time.Time
	if d := r.FormValue("due_date"); d != "" {
		if t, err := time.Parse("2006-01-02", d); err == nil {
			dueDate = &t
		}
	}
	inv.DueDate = dueDate

	// ── Update invoice fields ─────────────────────────────────────────
	override := r.FormValue("company_override") == "1"
	if override {
		inv.CompanyName = strings.TrimSpace(r.FormValue("company_name"))
		inv.CompanyEmail = strings.TrimSpace(r.FormValue("company_email"))
		inv.CompanyAddress = strings.TrimSpace(r.FormValue("company_address"))
		inv.CompanyCity = strings.TrimSpace(r.FormValue("company_city"))
		inv.CompanyZip = strings.TrimSpace(r.FormValue("company_zip"))
		inv.CompanyState = strings.TrimSpace(r.FormValue("company_state"))
		inv.CompanyCountry = strings.TrimSpace(r.FormValue("company_country"))
	} else {
		inv.CompanyName = ""
		inv.CompanyEmail = ""
		inv.CompanyAddress = ""
		inv.CompanyCity = ""
		inv.CompanyZip = ""
		inv.CompanyState = ""
		inv.CompanyCountry = ""
	}
	inv.ClientName = strings.TrimSpace(r.FormValue("client_name"))
	inv.ClientEmail = strings.TrimSpace(r.FormValue("client_email"))
	inv.ClientAddress = strings.TrimSpace(r.FormValue("client_address"))
	inv.ClientCity = strings.TrimSpace(r.FormValue("client_city"))
	inv.ClientZip = strings.TrimSpace(r.FormValue("client_zip"))
	inv.ClientState = strings.TrimSpace(r.FormValue("client_state"))
	inv.ClientCountry = strings.TrimSpace(r.FormValue("client_country"))
	inv.ShowLogo = r.FormValue("show_logo") == "on"
	inv.ShowTitle = r.FormValue("show_title") == "on"
	inv.AutoReminders = r.FormValue("auto_reminders") == "on"
	inv.LogoPosition = r.FormValue("logo_position")
	if inv.LogoPosition == "" {
		inv.LogoPosition = "left"
	}
	inv.TemplateID = r.FormValue("template_id")
	inv.BrandColor = r.FormValue("brand_color")
	inv.Currency = catalog.NormalizeCurrency(r.FormValue("currency"))
	taxRatePct, _ := strconv.ParseFloat(r.FormValue("tax_rate"), 64)
	inv.TaxRateBps = int64(math.Round(taxRatePct * 100))
	discountAmt, _ := strconv.ParseFloat(r.FormValue("discount_amount"), 64)
	inv.DiscountAmountCents = int64(math.Round(discountAmt * 100))
	inv.Notes = strings.TrimSpace(r.FormValue("notes"))
	inv.PaymentDetails = strings.TrimSpace(r.FormValue("payment_details"))

	service.NormalizeTemplateFields(inv, catalog.IsPaid(user.Plan))

	if err := h.App.InvRepo.UpdateInvoice(r.Context(), inv, items); err != nil {
		http.Error(w, "Update failed", http.StatusInternalServerError)
		return
	}

	audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
		UserID:     audit.UserIDPtr(user.ID),
		Action:     audit.ActionInvoiceUpdated,
		EntityType: audit.EntityInvoice,
		EntityID:   audit.EntityIDPtr(id),
		IPAddress:  audit.IPFromRequest(r),
	})
	http.Redirect(w, r, "/invoices/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// InvoiceStatusPost transitions an invoice to a new status.
// Cancels or schedules reminders based on the new status.
func (h *Handlers) InvoiceStatusPost(w http.ResponseWriter, r *http.Request) {
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

	inv, _, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil || !h.canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	newStatus := r.FormValue("status")

	paymentMethod := ""
	if newStatus == "paid" {
		paymentMethod = strings.ToLower(strings.TrimSpace(r.FormValue("payment_method")))
	}

	if err := h.App.InvRepo.UpdateInvoiceStatus(r.Context(), id, newStatus, paymentMethod, user.ID); err != nil {
		http.Error(w, "Failed to update status", http.StatusInternalServerError)
		return
	}

	// Cancel reminders when paid or voided.
	if newStatus == "paid" || newStatus == "void" {
		h.App.InvService.CancelReminders(r.Context(), id)
	}

	// Schedule reminders when marked sent.
	if newStatus == "sent" && inv.AutoReminders && inv.DueDate != nil {
		if err := h.App.InvService.ScheduleReminders(r.Context(), id, *inv.DueDate); err != nil {
			slog.Error("invoice reminder scheduling failed", "err", err, "invoice_id", id)
		}
	}
	audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
		UserID:     audit.UserIDPtr(user.ID),
		Action:     audit.ActionInvoiceStatusChanged,
		EntityType: audit.EntityInvoice,
		EntityID:   audit.EntityIDPtr(id),
		IPAddress:  audit.IPFromRequest(r),
		Metadata:   map[string]any{"new_status": newStatus},
	})
	redirectURL := fmt.Sprintf("/invoices/%d?status_changed=%s", id, newStatus)
	if newStatus == "paid" && paymentMethod != "" {
		redirectURL += "&method=" + paymentMethod
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// InvoiceDuplicateGet creates a copy of an existing invoice as a new draft.
func (h *Handlers) InvoiceDuplicateGet(w http.ResponseWriter, r *http.Request) {
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

	inv.ID = 0
	inv.InvoiceNumber = inv.InvoiceNumber + "-COPY"
	inv.Status = "draft"
	inv.CreatedAt = time.Now()
	inv.UpdatedAt = time.Now()

	newID, err := h.App.InvRepo.CreateInvoice(r.Context(), inv, items, "")
	if err != nil {
		http.Error(w, "Failed to duplicate invoice", http.StatusInternalServerError)
		return
	}
	audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
		UserID:     audit.UserIDPtr(user.ID),
		Action:     audit.ActionInvoiceDuplicated,
		EntityType: audit.EntityInvoice,
		EntityID:   audit.EntityIDPtr(newID),
		IPAddress:  audit.IPFromRequest(r),
		Metadata:   map[string]any{"source_id": id},
	})
	http.Redirect(w, r, "/invoices/"+strconv.FormatInt(newID, 10)+"/edit", http.StatusSeeOther)
}

// InvoiceDeletePost permanently deletes a draft invoice.
// Only draft invoices can be deleted — sent/paid/overdue invoices must be voided.
// Requires the user to confirm by typing the invoice number exactly.
func (h *Handlers) InvoiceDeletePost(w http.ResponseWriter, r *http.Request) {
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

	inv, _, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil || !h.canAccessInvoice(r, inv) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if inv.Status != "draft" {
		http.Error(w, "Only draft invoices can be deleted", http.StatusBadRequest)
		return
	}

	confirmation := strings.TrimSpace(r.FormValue("confirm_number"))
	if confirmation != inv.InvoiceNumber {
		http.Error(w, "Confirmation did not match invoice number", http.StatusBadRequest)
		return
	}

	if err := h.App.InvRepo.DeleteDraftInvoice(r.Context(), id, user.ID); err != nil {
		slog.Error("invoice delete failed", "err", err, "invoice_id", id, "user_id", user.ID)
		http.Error(w, "Could not delete invoice", http.StatusInternalServerError)
		return
	}
	audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
		UserID:     audit.UserIDPtr(user.ID),
		Action:     audit.ActionInvoiceDeleted,
		EntityType: audit.EntityInvoice,
		EntityID:   audit.EntityIDPtr(id),
		IPAddress:  audit.IPFromRequest(r),
		Metadata:   map[string]any{"invoice_number": inv.InvoiceNumber},
	})

	slog.Info("invoice deleted", "invoice_id", id, "user_id", user.ID)
	http.Redirect(w, r, "/invoices?deleted=true", http.StatusSeeOther)
}


// InvoiceReportPaymentPost handles clients reporting that they sent Zelle or check payment.
// This notifies the owner and updates the invoice view state cleanly.
func (h *Handlers) InvoiceReportPaymentPost(w http.ResponseWriter, r *http.Request) {
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

	// Security validation: match public token
	accessToken := r.FormValue("access")
	if accessToken == "" {
		accessToken = r.URL.Query().Get("access")
	}
	if inv.PublicToken != "" && accessToken != inv.PublicToken && auth.GetUser(r) == nil {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	method := strings.TrimSpace(r.FormValue("payment_method"))
	note := strings.TrimSpace(r.FormValue("reported_note"))
	if method == "" {
		http.Error(w, "Payment method is required", http.StatusBadRequest)
		return
	}

	// Update database columns
	if err := h.App.InvRepo.(*repo.InvoiceRepo).ReportPaymentSent(r.Context(), id, method, note); err != nil {
		slog.Error("failed to update reported payment info", "invoice_id", id, "err", err)
		http.Error(w, "Database update failed", http.StatusInternalServerError)
		return
	}

	// Send notification email to the owner
	if inv.UserID != nil {
		owner, err := h.App.UserRepo.GetByID(*inv.UserID)
		if err == nil && owner != nil {
			currencySym := "USD"
			if inv.Currency != "" {
				currencySym = inv.Currency
			}

			verifyURL := fmt.Sprintf("%s/invoices/%d", h.App.BaseURL, inv.ID)
			
			// Queue resend mailer
			_ = h.App.Mailer.SendPaymentReportedNotification(owner.Email, mailer.PaymentReportedEmailData{
				InvoiceNumber: inv.InvoiceNumber,
				ClientName:    inv.ClientName,
				CompanyName:   inv.CompanyName,
				Amount:        fmt.Sprintf("%.2f", float64(inv.TotalCents)/100.0),
				Currency:      currencySym,
				Method:        strings.ToUpper(method),
				Note:          note,
				VerifyURL:     verifyURL,
			})
		}
	}

	// Success response back to invoice page
	redirectURL := fmt.Sprintf("/invoices/%d?reported=1", id)
	if accessToken != "" {
		redirectURL += "&access=" + accessToken
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}
