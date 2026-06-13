// internal/handlers/invoice.go
package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/pdf"
	"psiloconvalley/internal/repo"
	"psiloconvalley/internal/service"
	"psiloconvalley/internal/views"
)

// =====================================================================
// PUBLIC / FREEMIUM INVOICE ROUTES
//
// Anonymous users may create/view ONLY invoices matching their anon token.
// Logged-in users may view ONLY invoices matching their user_id.
// =====================================================================

func (h *Handlers) InvoiceCreatePost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	user := auth.GetUser(r)
	if user != nil && h.hasReachedLimit(r) {
		http.Redirect(w, r, "/pricing?reason=invoice-limit", http.StatusSeeOther)
		return
	}

	// ── Anonymous token handling ─────────────────────────────────────
	var anonymousToken string
	if user == nil {
		if auth.AnonLimitReached(r) {
			http.Redirect(w, r, "/register?reason=limit", http.StatusSeeOther)
			return
		}
		existingToken, hasToken := auth.GetAnonymousToken(r)
		if hasToken && existingToken != "" {
			anonymousToken = existingToken
		} else {
			token, err := auth.GenerateToken(32)
			if err != nil {
				http.Error(w, "Failed to generate token", http.StatusInternalServerError)
				return
			}
			anonymousToken = token
			auth.SetAnonymousToken(w, anonymousToken)
		}
	}

	// ── Parse all form fields ────────────────────────────────────────
	clientName := strings.TrimSpace(r.FormValue("client_name"))
	companyName := strings.TrimSpace(r.FormValue("company_name"))
	invoiceNumber := strings.TrimSpace(r.FormValue("invoice_number"))
	currency := catalog.NormalizeCurrency(r.FormValue("currency"))

	// ── Build line items via service ─────────────────────────────────
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
		errs = append(errs, FormError{Field: "client_name", Message: "Customer name is required (at least 2 characters)"})
	}
	if companyName == "" {
		errs = append(errs, FormError{Field: "company_name", Message: "Company name is required"})
	}
	if len(items) == 0 {
		errs = append(errs, FormError{Field: "items", Message: "At least one line item with a description is required"})
	}
	if invoiceNumber != "" {
		exists, err := h.App.InvRepo.InvoiceNumberExists(r.Context(), invoiceNumber, user.ID)
		if err != nil {
			log.Printf("[invoice] invoice number exists check error: %v", err)
			http.Error(w, "Failed to validate invoice number", http.StatusInternalServerError)
			return
		}
		if exists {
			errs = append(errs, FormError{
				Field:   "invoice_number",
				Message: "Invoice number already exists. Choose a different one or leave it blank to auto-generate.",
			})
		}
	}

	if len(errs) > 0 {
		data := map[string]any{
			"User":       user,
			"IsLoggedIn": user != nil,
			"Mode":       "create",
			"DocumentType": "invoice",
			"Currencies": catalog.SupportedCurrencies,
			"USStates":   catalog.USStates,
			"Templates":  catalog.InvoiceTemplates,
			"Errors":     errs,
			"Invoice": views.InvoicePage{
				CompanyName:    companyName,
				CompanyEmail:   r.FormValue("company_email"),
				CompanyAddress: r.FormValue("company_address"),
				CompanyCity:    r.FormValue("company_city"),
				CompanyZip:     r.FormValue("company_zip"),
				CompanyState:   r.FormValue("company_state"),
				CompanyCountry: r.FormValue("company_country"),
				ClientName:     clientName,
				ClientEmail:    r.FormValue("client_email"),
				ClientAddress:  r.FormValue("client_address"),
				ClientCity:     r.FormValue("client_city"),
				ClientZip:      r.FormValue("client_zip"),
				ClientState:    r.FormValue("client_state"),
				ClientCountry:  r.FormValue("client_country"),
				InvoiceNumber:  invoiceNumber,
				Currency:       currency,
				Notes:          r.FormValue("notes"),
				PaymentDetails: r.FormValue("payment_details"),
				ShowLogo:       r.FormValue("show_logo") == "on",
				ShowTitle:      r.FormValue("show_title") == "on",
				LogoPosition:   r.FormValue("logo_position"),
				AutoReminders:  r.FormValue("auto_reminders") == "on",
				LogoURL: func() template.URL {
					if user != nil {
						if bp, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID); err == nil && bp != nil {
							return template.URL(bp.LogoURL)
						}
					}
					return ""
				}(),
			},
		}
		if user != nil {
			clients, err := h.App.ClientRepo.ListByUserID(r.Context(), user.ID)
			if err == nil && len(clients) > 0 {
				data["Clients"] = clients
			}
		}
		h.App.Render(w, r, "invoice_new.tmpl", data)
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

	// ── Tax rate (percent → bps) ─────────────────────────────────────
	taxRatePct, _ := strconv.ParseFloat(r.FormValue("tax_rate"), 64)
	taxRateBps := int64(math.Round(taxRatePct * 100))
	// ── Discount (dollars → cents) ───────────────────────────────────
	discountAmt, _ := strconv.ParseFloat(r.FormValue("discount_amount"), 64)
	discountCents := int64(math.Round(discountAmt * 100))
	// ── Appearance toggles ───────────────────────────────────────────
	showLogo := r.FormValue("show_logo") == "on"
	showTitle := r.FormValue("show_title") == "on"
	autoReminders := r.FormValue("auto_reminders") == "on"
	logoPosition := r.FormValue("logo_position")
	if logoPosition == "" {
		logoPosition = "left"
	}
	templateID := r.FormValue("template_id")
	brandColor := r.FormValue("brand_color")

	// ── Auto-generate invoice number if blank ─────────────────────────
	if invoiceNumber == "" {
		if user != nil {
			num, err := h.App.UserRepo.NextInvoiceNumber(r.Context(), user.ID)
			if err != nil {
				log.Printf("[invoice] failed to generate invoice number: %v", err)
				invoiceNumber = fmt.Sprintf("INV-%d", time.Now().UnixNano())
			} else {
				invoiceNumber = num
			}
		} else {
			invoiceNumber = fmt.Sprintf("INV-%d", time.Now().UnixNano())
		}
	}

	// ── Build invoice ────────────────────────────────────────────────
	var userID *int64
	var bizProfileID *int64
	if user != nil {
		userID = &user.ID
		bp, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID)
		if err == nil {
			bizProfileID = &bp.ID
		}
	}

	inv := &repo.Invoice{
		UserID:              userID,
		BusinessProfileID:   bizProfileID,
		AnonymousToken:      anonymousToken,
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
		ShowLogo:            showLogo,
		ShowTitle:           showTitle,
		AutoReminders:       autoReminders,
		TemplateID:          templateID,
		BrandColor:          brandColor,
		LogoPosition:        logoPosition,
		Currency:            currency,
		Notes:               strings.TrimSpace(r.FormValue("notes")),
		PaymentDetails:      strings.TrimSpace(r.FormValue("payment_details")),
		Status:              "draft",
	}

	isPro := user != nil && user.Plan == "pro"
	service.NormalizeTemplateFields(inv, isPro)

	invoiceID, err := h.App.InvRepo.CreateInvoice(r.Context(), inv, items, anonymousToken)
	if err != nil {
		log.Printf("[invoice] create error: %v", err)

		// Catch duplicate invoice number from DB constraint
		if strings.Contains(err.Error(), "invoices_invoice_number_key") ||
			strings.Contains(err.Error(), "idx_invoices_user_invoice_number") {
			dupErrs := []struct {
				Field   string
				Message string
			}{
				{Field: "invoice_number", Message: "Invoice number already exists. Please choose a different one or leave it blank to auto-generate."},
			}
			data := map[string]any{
				"User":         user,
				"IsLoggedIn":   user != nil,
				"Mode":         "create",
				"DocumentType": "invoice",
				"Currencies":   catalog.SupportedCurrencies,
				"USStates":     catalog.USStates,
				"Templates":    catalog.InvoiceTemplates,
				"Errors":       dupErrs,
				"Invoice": views.InvoicePage{
					CompanyName:    inv.CompanyName,
					CompanyEmail:   inv.CompanyEmail,
					CompanyAddress: inv.CompanyAddress,
					CompanyCity:    inv.CompanyCity,
					CompanyZip:     inv.CompanyZip,
					CompanyState:   inv.CompanyState,
					CompanyCountry: inv.CompanyCountry,
					ClientName:     inv.ClientName,
					ClientEmail:    inv.ClientEmail,
					ClientAddress:  inv.ClientAddress,
					ClientCity:     inv.ClientCity,
					ClientZip:      inv.ClientZip,
					ClientState:    inv.ClientState,
					ClientCountry:  inv.ClientCountry,
					InvoiceNumber:  inv.InvoiceNumber,
					Currency:       inv.Currency,
					Notes:          inv.Notes,
					PaymentDetails: inv.PaymentDetails,
					ShowLogo:       inv.ShowLogo,
					ShowTitle:      inv.ShowTitle,
					AutoReminders:  inv.AutoReminders,
					LogoURL: func() template.URL {
						if user != nil {
							if bp, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID); err == nil && bp != nil {
								return template.URL(bp.LogoURL)
							}
						}
						return ""
					}(),
				},
			}
			if user != nil {
				clients, err := h.App.ClientRepo.ListByUserID(r.Context(), user.ID)
				if err == nil && len(clients) > 0 {
					data["Clients"] = clients
				}
			}
			h.App.Render(w, r, "invoice_new.tmpl", data)
			return
		}

		http.Error(w, "Failed to create invoice", http.StatusInternalServerError)
		return
	}

	// ── Increment anon counter after successful save ──────────────────
	if user == nil {
		count := auth.GetAnonInvoiceCount(r)
		auth.SetAnonInvoiceCount(w, count+1)
	}

	// ── Create recurring schedule if enabled ─────────────────────────
	if user != nil && r.FormValue("is_recurring") == "on" {
		err := h.App.InvService.CreateRecurringSchedule(r.Context(), service.ScheduleRecurringParams{
			UserID:    user.ID,
			InvoiceID: invoiceID,
			Frequency: r.FormValue("recurring_frequency"),
			AutoSend:  r.FormValue("recurring_auto_send") == "on",
		})
		if err != nil {
			log.Printf("[invoice] failed to create recurring schedule: %v", err)
		}
	}

	http.Redirect(w, r, "/invoices/"+strconv.FormatInt(invoiceID, 10), http.StatusSeeOther)
}

func (h *Handlers) InvoicePDFGet(w http.ResponseWriter, r *http.Request) {
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

	if !h.canAccessInvoice(r, inv) {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	user := auth.GetUser(r)
	invoiceView := views.MapInvoicePage(inv, items, "view")

	// PDF mode: inline logo as base64 — headless Chrome cannot fetch external images.
	logoStr := string(invoiceView.LogoURL)
	if logoStr != "" {
		if strings.HasPrefix(logoStr, "/static/") {
			filePath := "." + logoStr
			if b, err := os.ReadFile(filePath); err == nil {
				mime := http.DetectContentType(b)
				invoiceView.LogoURL = template.URL("data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b))
			} else {
				log.Printf("[pdf] logo read from disk error for invoice %d: %v", id, err)
			}
		} else if strings.HasPrefix(logoStr, "http") {
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(logoStr)
			if err != nil {
				log.Printf("[pdf] logo fetch error for invoice %d: %v", id, err)
			} else {
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					log.Printf("[pdf] logo fetch non-200 for invoice %d: status=%d url=%s", id, resp.StatusCode, logoStr)
				} else {
					b, err := io.ReadAll(resp.Body)
					if err != nil {
						log.Printf("[pdf] logo read error for invoice %d: %v", id, err)
					} else {
						mime := http.DetectContentType(b)
						invoiceView.LogoURL = template.URL("data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b))
						log.Printf("[pdf] logo inlined as base64 for invoice %d (%d bytes)", id, len(b))
					}
				}
			}
		}
	}

	// Branding footer: hidden for Growth and Pro plans.
	showBranding := true
	if inv.UserID != nil {
		if owner, err := h.App.UserRepo.GetByID(*inv.UserID); err == nil && owner != nil {
			if owner.Plan == "pro" || owner.Plan == "growth" {
				showBranding = false
			}
		}
	}

	var buf bytes.Buffer
	templateData := map[string]any{
		"Invoice":      invoiceView,
		"User":         user,
		"csrfField":    "",
		"ShowBranding": showBranding,
	}

	if err := h.App.Templates.ExecuteTemplate(&buf, service.InvoiceTemplateName(invoiceView.TemplateID), templateData); err != nil {
		http.Error(w, "Could not render invoice", http.StatusInternalServerError)
		return
	}

	pdfCtx, pdfCancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer pdfCancel()

	pdfBytes, err := pdf.Generate(pdfCtx, buf.String())
	if err != nil {
		http.Error(w, "Could not generate PDF", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=invoice-"+strconv.FormatInt(id, 10)+".pdf")
	_, _ = w.Write(pdfBytes)
}

// =====================================================================
// PROTECTED INVOICE MANAGEMENT ROUTES
// All routes below require auth.RequireAuth (enforced in router.go).
// canAccessInvoice provides a secondary user_id ownership check.
// =====================================================================

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

	// ── Parse form fields ─────────────────────────────────────────────
	currency := catalog.NormalizeCurrency(r.FormValue("currency"))
	taxRatePct, _ := strconv.ParseFloat(r.FormValue("tax_rate"), 64)
	taxRateBps := int64(math.Round(taxRatePct * 100))
	discountAmt, _ := strconv.ParseFloat(r.FormValue("discount_amount"), 64)
	discountCents := int64(math.Round(discountAmt * 100))

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
	inv.ShowLogo = r.FormValue("show_logo") == "on"
	inv.ShowTitle = r.FormValue("show_title") == "on"
	inv.AutoReminders = r.FormValue("auto_reminders") == "on"
	inv.LogoPosition = r.FormValue("logo_position")
	if inv.LogoPosition == "" {
		inv.LogoPosition = "left"
	}
	inv.TemplateID = r.FormValue("template_id")
	inv.BrandColor = r.FormValue("brand_color")
	inv.Currency = currency
	inv.TaxRateBps = taxRateBps
	inv.DiscountAmountCents = discountCents
	inv.Notes = strings.TrimSpace(r.FormValue("notes"))
	inv.PaymentDetails = strings.TrimSpace(r.FormValue("payment_details"))

	isPro := user != nil && user.Plan == "pro"
	service.NormalizeTemplateFields(inv, isPro)

	if err := h.App.InvRepo.UpdateInvoice(r.Context(), inv, items); err != nil {
		http.Error(w, "Update failed", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/invoices/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

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

	if err := h.App.InvRepo.UpdateInvoiceStatus(r.Context(), id, newStatus, user.ID); err != nil {
		http.Error(w, "Failed to update status", http.StatusInternalServerError)
		return
	}

	// ── Cancel reminders when paid or voided ─────────────────────────
	if newStatus == "paid" || newStatus == "void" {
		h.App.InvService.CancelReminders(r.Context(), id)
	}

	// ── Schedule reminders when marked sent ──────────────────────────
	if newStatus == "sent" && inv.AutoReminders && inv.DueDate != nil {
		if err := h.App.InvService.ScheduleReminders(r.Context(), id, *inv.DueDate); err != nil {
			log.Printf("[invoice] failed to schedule reminders for invoice %d: %v", id, err)
		}
	}

	http.Redirect(w, r, "/invoices/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

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
		log.Printf("[invoice] delete error: %v", err)
		http.Error(w, "Could not delete invoice", http.StatusInternalServerError)
		return
	}

	log.Printf("[invoice] draft invoice %d deleted by user %d", id, user.ID)
	http.Redirect(w, r, "/invoices?deleted=true", http.StatusSeeOther)
}
