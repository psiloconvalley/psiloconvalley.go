// internal/handlers/estimate_write.go
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
	"psiloconvalley/internal/repo"
	"psiloconvalley/internal/service"
	"psiloconvalley/internal/views"
)

// EstimateCreatePost handles the estimate creation form submission.
func (h *Handlers) EstimateCreatePost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !h.canCreateEstimate(r) {
		http.Redirect(w, r, "/pricing?reason=estimate-limit", http.StatusSeeOther)
		return
	}

	clientName := strings.TrimSpace(r.FormValue("client_name"))
	companyName := strings.TrimSpace(r.FormValue("company_name"))
	invoiceNumber := strings.TrimSpace(r.FormValue("invoice_number"))
	currency := catalog.NormalizeCurrency(r.FormValue("currency"))

	items := service.ParseLineItems(service.LineItemInput{
		Descriptions: r.Form["description[]"],
		Details:      r.Form["details[]"],
		Quantities:   r.Form["quantity[]"],
		UnitPrices:   r.Form["unit_price[]"],
	})

	// ── Validation ───────────────────────────────────────────────────
	type FormError struct {
		Field   string
		Message string
	}
	var errs []FormError

	if len(clientName) < 2 {
		errs = append(errs, FormError{Field: "client_name", Message: "Client name is required"})
	}
	if companyName == "" {
		errs = append(errs, FormError{Field: "company_name", Message: "Company name is required"})
	}
	if len(items) == 0 {
		errs = append(errs, FormError{Field: "items", Message: "At least one line item is required"})
	}

	if len(errs) > 0 {
		h.App.Render(w, r, "invoice_new.tmpl", map[string]any{
			"User":         user,
			"IsLoggedIn":   true,
			"Mode":         "create",
			"DocumentType": "estimate",
			"Currencies":   catalog.SupportedCurrencies,
			"USStates":     catalog.USStates,
			"Templates":    catalog.InvoiceTemplates,
			"Errors":       errs,
			"Invoice": views.InvoicePage{
				CompanyName:   companyName,
				ClientName:    clientName,
				InvoiceNumber: invoiceNumber,
				Currency:      currency,
			},
		})
		return
	}

	// ── Parse dates ──────────────────────────────────────────────────
	issueDate := time.Now()
	if d := r.FormValue("issue_date"); d != "" {
		if t, err := time.Parse("2006-01-02", d); err == nil {
			issueDate = t
		}
	}

	var dueDate *time.Time
	if d := r.FormValue("due_date"); d != "" {
		if t, err := time.Parse("2006-01-02", d); err == nil {
			dueDate = &t
		}
	}

	// ── Numeric fields ───────────────────────────────────────────────
	taxRatePct, _ := strconv.ParseFloat(r.FormValue("tax_rate"), 64)
	taxRateBps := int64(math.Round(taxRatePct * 100))
	discountAmt, _ := strconv.ParseFloat(r.FormValue("discount_amount"), 64)
	discountCents := int64(math.Round(discountAmt * 100))
	logoPosition := r.FormValue("logo_position")
	if logoPosition == "" {
		logoPosition = "left"
	}

	// ── Auto-generate estimate number ─────────────────────────────────
	if invoiceNumber == "" {
		num, err := h.App.UserRepo.NextEstimateNumber(r.Context(), user.ID)
		if err != nil {
			slog.Error("estimate number generation failed", "err", err)
			invoiceNumber = fmt.Sprintf("EST-%d", time.Now().UnixNano())
		} else {
			invoiceNumber = num
		}
	}

	var bizProfileID *int64
	if bp, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID); err == nil && bp != nil {
		bizProfileID = &bp.ID
	}

	inv := &repo.Invoice{
		UserID:              &user.ID,
		BusinessProfileID:   bizProfileID,
		CompanyName:         companyName,
		CompanyEmail:        strings.TrimSpace(r.FormValue("company_email")),
		CompanyAddress:      strings.TrimSpace(r.FormValue("company_address")),
		CompanyCity:         strings.TrimSpace(r.FormValue("company_city")),
		CompanyZip:          strings.TrimSpace(r.FormValue("company_zip")),
		CompanyState:        strings.TrimSpace(r.FormValue("company_state")),
		CompanyCountry:      strings.TrimSpace(r.FormValue("company_country")),
		ClientName:          clientName,
		ClientEmail:         strings.TrimSpace(r.FormValue("client_email")),
		ClientAddress:       strings.TrimSpace(r.FormValue("client_address")),
		ClientCity:          strings.TrimSpace(r.FormValue("client_city")),
		ClientZip:           strings.TrimSpace(r.FormValue("client_zip")),
		ClientState:         strings.TrimSpace(r.FormValue("client_state")),
		ClientCountry:       strings.TrimSpace(r.FormValue("client_country")),
		InvoiceNumber:       invoiceNumber,
		IssueDate:           issueDate,
		DueDate:             dueDate,
		TaxRateBps:          taxRateBps,
		DiscountAmountCents: discountCents,
		ShowLogo:            r.FormValue("show_logo") == "on",
		ShowTitle:           r.FormValue("show_title") == "on",
		TemplateID:          r.FormValue("template_id"),
		BrandColor:          r.FormValue("brand_color"),
		LogoPosition:        logoPosition,
		Currency:            currency,
		Notes:               strings.TrimSpace(r.FormValue("notes")),
		PaymentDetails:      strings.TrimSpace(r.FormValue("payment_details")),
		Status:              "draft",
		DocumentType:        "estimate",
	}

	service.NormalizeTemplateFields(inv, catalog.IsPaid(user.Plan))

	estimateID, err := h.App.InvRepo.CreateInvoice(r.Context(), inv, items, "")
	if err != nil {
		slog.Error("estimate create failed", "err", err)
		http.Error(w, "Failed to create estimate", http.StatusInternalServerError)
		return
	}
	if err := h.App.UsageRepo.Increment(r.Context(), user.ID, "estimates"); err != nil {
    		slog.Warn("estimate usage increment failed", "err", err)
	}
	audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
		UserID:     audit.UserIDPtr(user.ID),
		Action:     audit.ActionEstimateCreated,
		EntityType: audit.EntityEstimate,
		EntityID:   audit.EntityIDPtr(estimateID),
		IPAddress:  audit.IPFromRequest(r),
		Metadata:   map[string]any{"estimate_number": inv.InvoiceNumber},
	})

	http.Redirect(w, r, "/estimates/"+strconv.FormatInt(estimateID, 10), http.StatusSeeOther)
}

// EstimateEditPost updates an existing estimate.
func (h *Handlers) EstimateEditPost(w http.ResponseWriter, r *http.Request) {
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

	if inv.DocumentType != "estimate" {
		http.NotFound(w, r)
		return
	}

	items := service.ParseLineItems(service.LineItemInput{
		Descriptions: r.Form["description[]"],
		Details:      r.Form["details[]"],
		Quantities:   r.Form["quantity[]"],
		UnitPrices:   r.Form["unit_price[]"],
	})

	if len(items) == 0 {
		http.Error(w, "At least one line item is required", http.StatusBadRequest)
		return
	}

	// ── Parse dates ──────────────────────────────────────────────────
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

	invoiceNumber := strings.TrimSpace(r.FormValue("invoice_number"))
	if invoiceNumber == "" {
		invoiceNumber = inv.InvoiceNumber
	}

	// ── Update fields ─────────────────────────────────────────────────
	inv.CompanyName = strings.TrimSpace(r.FormValue("company_name"))
	inv.CompanyEmail = strings.TrimSpace(r.FormValue("company_email"))
	inv.CompanyAddress = strings.TrimSpace(r.FormValue("company_address"))
	inv.CompanyCity = strings.TrimSpace(r.FormValue("company_city"))
	inv.CompanyZip = strings.TrimSpace(r.FormValue("company_zip"))
	inv.CompanyState = strings.TrimSpace(r.FormValue("company_state"))
	inv.CompanyCountry = strings.TrimSpace(r.FormValue("company_country"))
	inv.ClientName = strings.TrimSpace(r.FormValue("client_name"))
	inv.ClientEmail = strings.TrimSpace(r.FormValue("client_email"))
	inv.ClientAddress = strings.TrimSpace(r.FormValue("client_address"))
	inv.ClientCity = strings.TrimSpace(r.FormValue("client_city"))
	inv.ClientZip = strings.TrimSpace(r.FormValue("client_zip"))
	inv.ClientState = strings.TrimSpace(r.FormValue("client_state"))
	inv.ClientCountry = strings.TrimSpace(r.FormValue("client_country"))
	inv.InvoiceNumber = invoiceNumber
	inv.Notes = strings.TrimSpace(r.FormValue("notes"))
	inv.PaymentDetails = strings.TrimSpace(r.FormValue("payment_details"))
	inv.Currency = catalog.NormalizeCurrency(r.FormValue("currency"))
	inv.ShowLogo = r.FormValue("show_logo") == "on"
	inv.ShowTitle = r.FormValue("show_title") == "on"
	inv.TemplateID = r.FormValue("template_id")
	inv.BrandColor = r.FormValue("brand_color")
	inv.LogoPosition = r.FormValue("logo_position")
	if inv.LogoPosition == "" {
		inv.LogoPosition = "left"
	}
	taxRatePct, _ := strconv.ParseFloat(r.FormValue("tax_rate"), 64)
	inv.TaxRateBps = int64(taxRatePct * 100)
	discountAmt, _ := strconv.ParseFloat(r.FormValue("discount_amount"), 64)
	inv.DiscountAmountCents = int64(math.Round(discountAmt * 100))
	service.NormalizeTemplateFields(inv, catalog.IsPaid(user.Plan))

	if err := h.App.InvRepo.UpdateInvoice(r.Context(), inv, items); err != nil {
		slog.Error("estimate update failed", "err", err)
		http.Error(w, "Failed to update estimate", http.StatusInternalServerError)
		return
	}
	audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
		UserID:     audit.UserIDPtr(user.ID),
		Action:     audit.ActionEstimateUpdated,
		EntityType: audit.EntityEstimate,
		EntityID:   audit.EntityIDPtr(id),
		IPAddress:  audit.IPFromRequest(r),
	})
	http.Redirect(w, r, fmt.Sprintf("/estimates/%d", id), http.StatusSeeOther)
}

// EstimateStatusPost transitions an estimate to a new status.
func (h *Handlers) EstimateStatusPost(w http.ResponseWriter, r *http.Request) {
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

	if inv.DocumentType != "estimate" {
		http.Error(w, "Not an estimate", http.StatusBadRequest)
		return
	}

	newStatus := r.FormValue("status")
	if err := h.App.InvRepo.UpdateEstimateStatus(r.Context(), id, user.ID, newStatus); err != nil {
		slog.Error("estimate status update failed", "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
		UserID:     audit.UserIDPtr(user.ID),
		Action:     audit.ActionEstimateStatusChanged,
		EntityType: audit.EntityEstimate,
		EntityID:   audit.EntityIDPtr(id),
		IPAddress:  audit.IPFromRequest(r),
		Metadata:   map[string]any{"new_status": newStatus},
	})
	http.Redirect(w, r, fmt.Sprintf("/estimates/%d", id), http.StatusSeeOther)
}

// EstimateDeletePost permanently deletes a draft estimate.
func (h *Handlers) EstimateDeletePost(w http.ResponseWriter, r *http.Request) {
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

	if inv.DocumentType != "estimate" {
		http.Error(w, "Not an estimate", http.StatusBadRequest)
		return
	}

	if inv.Status != "draft" {
		http.Error(w, "Only draft estimates can be deleted", http.StatusBadRequest)
		return
	}

	confirmation := strings.TrimSpace(r.FormValue("confirm_number"))
	if confirmation != inv.InvoiceNumber {
		http.Error(w, "Confirmation did not match estimate number", http.StatusBadRequest)
		return
	}

	if err := h.App.InvRepo.DeleteDraftInvoice(r.Context(), id, user.ID); err != nil {
		slog.Error("estimate delete failed", "err", err)
		http.Error(w, "Could not delete estimate", http.StatusInternalServerError)
		return
	}

	slog.Info("estimate deleted", "estimate_id", id, "user_id", user.ID)
	audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
		UserID:     audit.UserIDPtr(user.ID),
		Action:     audit.ActionEstimateDeleted,
		EntityType: audit.EntityEstimate,
		EntityID:   audit.EntityIDPtr(id),
		IPAddress:  audit.IPFromRequest(r),
		Metadata:   map[string]any{"estimate_number": inv.InvoiceNumber},
	})
	http.Redirect(w, r, "/estimates?deleted=true", http.StatusSeeOther)
}

// EstimateConvertPost converts an accepted estimate into a draft invoice.
func (h *Handlers) EstimateConvertPost(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if inv.DocumentType != "estimate" {
		http.Error(w, "Not an estimate", http.StatusBadRequest)
		return
	}

	newID, err := h.App.InvService.ConvertEstimateToInvoice(r.Context(), service.ConvertEstimateParams{
		UserID:   user.ID,
		Estimate: inv,
		Items:    items,
	})
	if err != nil {
		slog.Error("estimate convert failed", "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
		audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
		UserID:     audit.UserIDPtr(user.ID),
		Action:     audit.ActionEstimateConverted,
		EntityType: audit.EntityEstimate,
		EntityID:   audit.EntityIDPtr(id),
		IPAddress:  audit.IPFromRequest(r),
		Metadata:   map[string]any{"new_invoice_id": newID},
	})
	http.Redirect(w, r, fmt.Sprintf("/invoices/%d", newID), http.StatusSeeOther)
}

