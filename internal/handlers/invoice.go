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
	"psiloconvalley/internal/views"
)

// =====================================================================
// PUBLIC / FREEMIUM INVOICE ROUTES
//
// Anonymous users may create/view ONLY invoices matching their anon token.
// Logged-in users may view ONLY invoices matching their user_id.
// =====================================================================

func (h *Handlers) InvoiceNewGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)

	if user == nil && auth.AnonLimitReached(r) {
		http.Redirect(w, r, "/register?reason=limit", http.StatusSeeOther)
		return
	}
	if user != nil && h.hasReachedLimit(r) {
		http.Redirect(w, r, "/pricing?reason=invoice-limit", http.StatusSeeOther)
		return
	}

	invoiceData := views.InvoicePage{
		CompanyCountry: "United States",
		CompanyState:   "California",
		ClientCountry:  "United States",
		ClientState:    "California",
		ShowLogo:       true,
		ShowTitle:      true,
		TemplateID:     catalog.DefaultTemplateID,
		BrandColor:     catalog.DefaultBrandColor,
		LogoPosition:   "left",
	}
	if user != nil {
	if bp, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID); err == nil && bp != nil {
		invoiceData.LogoURL      = template.URL(bp.LogoURL)
		invoiceData.CompanyName    = bp.Name
		invoiceData.CompanyEmail   = bp.Email
		invoiceData.CompanyAddress = bp.Address
		invoiceData.CompanyCity    = bp.City
		invoiceData.CompanyState   = bp.State
		invoiceData.CompanyZip     = bp.Zip
		invoiceData.CompanyCountry = bp.Country
	}

}
		data := map[string]any{
		"User":       user,
		"IsLoggedIn": user != nil,
		"Invoice":    invoiceData,
		"Mode":       "create",
		"Currencies": catalog.SupportedCurrencies,
		"USStates":   catalog.USStates,
		"Templates":  catalog.InvoiceTemplates,
	}
	// Logged-in users get their client dropdown populated
	if user != nil {
		clients, err := h.App.ClientRepo.ListByUserID(r.Context(), user.ID)
		if err == nil && len(clients) > 0 {
			data["Clients"] = clients
		}
	}

	h.App.Render(w, r, "invoice_new.tmpl", data)
}
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

	descriptions := r.Form["description[]"]
	details := r.Form["details[]"]
	quantities := r.Form["quantity[]"]
	unitPrices := r.Form["unit_price[]"]

	// ── Build line items ─────────────────────────────────────────────
	var items []repo.InvoiceItem
	for i, desc := range descriptions {
		desc = strings.TrimSpace(desc)
		if desc == "" {
			continue
		}
		qty := float64(1)
		if i < len(quantities) {
			qty, _ = strconv.ParseFloat(quantities[i], 64)
		}
		if qty <= 0 {
			qty = 1
		}
		var unitPrice float64
		if i < len(unitPrices) {
			unitPrice, _ = strconv.ParseFloat(unitPrices[i], 64)
		}
		detail := ""
		if i < len(details) {
			detail = strings.TrimSpace(details[i])
		}
		unitPriceCents := int64(math.Round(unitPrice * 100))
		items = append(items, repo.InvoiceItem{
			Description:    desc,
			Details:        detail,
			Quantity:       qty,
			UnitPriceCents: unitPriceCents,
		})
	}

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
		exists, err := h.App.InvRepo.InvoiceNumberExists(r.Context(), invoiceNumber)
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
		TemplateID:	     templateID,
		BrandColor:	     brandColor,
		LogoPosition:        logoPosition,
		Currency:            currency,
		Notes:               strings.TrimSpace(r.FormValue("notes")),
		PaymentDetails:      strings.TrimSpace(r.FormValue("payment_details")),
		Status:              "draft",
	}
	isPro := user != nil && user.Plan == "pro"
	normalizeTemplateFields(inv, isPro)
	invoiceID, err := h.App.InvRepo.CreateInvoice(r.Context(), inv, items, anonymousToken)
	if err != nil {
		log.Printf("[invoice] create error: %v", err)

		// Catch duplicate invoice number from DB constraint
		if strings.Contains(err.Error(), "invoices_invoice_number_key") {
			dupErrs := []struct {
				Field   string
				Message string
			}{
				{Field: "invoice_number", Message: "Invoice number already exists. Please choose a different one or leave it blank to auto-generate."},
			}
			data := map[string]any{
				"User":       user,
				"IsLoggedIn": user != nil,
				"Mode":       "create",
				"Currencies": catalog.SupportedCurrencies,
				"USStates":   catalog.USStates,
				"Templates":  catalog.InvoiceTemplates,
				"Errors":     dupErrs,
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
	// Increment anon counter after successful save
	// Increment anon counter after successful save
	if user == nil {
		count := auth.GetAnonInvoiceCount(r)
		auth.SetAnonInvoiceCount(w, count+1)
	}

	// ── Create recurring schedule if enabled ─────────────────────────
	if user != nil && r.FormValue("is_recurring") == "on" {
		frequency := r.FormValue("recurring_frequency")
		if frequency == "" {
			frequency = "monthly"
		}
		autoSend := r.FormValue("recurring_auto_send") == "on"

		nextRun := calculateNextRun(frequency, time.Now())

		sched := &repo.RecurringSchedule{
			UserID:            user.ID,
			TemplateInvoiceID: invoiceID,
			Frequency:         frequency,
			SendAutomatically: autoSend,
			Active:            true,
			NextRunAt:         nextRun,
		}

		schedID, err := h.App.SchedulerRepo.CreateRecurringSchedule(r.Context(), sched)
		if err != nil {
			log.Printf("[invoice] failed to create recurring schedule: %v", err)
		} else {
			// Schedule the first job
			payload := map[string]any{"schedule_id": schedID}
			jobID, err := h.App.SchedulerRepo.CreateJob(r.Context(), "generate_recurring_invoice", payload, nextRun)
			if err != nil {
				log.Printf("[invoice] failed to schedule first recurring job: %v", err)
			} else {
				log.Printf("[invoice] recurring schedule %d created for invoice %d (first run job %d at %s)",
					schedID, invoiceID, jobID, nextRun.Format("2006-01-02"))
			}
		}
	}

	http.Redirect(w, r, "/invoices/"+strconv.FormatInt(invoiceID, 10), http.StatusSeeOther)
}
func (h *Handlers) InvoiceDetail(w http.ResponseWriter, r *http.Request) {
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

	if !h.canViewInvoice(r, inv) {
		http.Error(w, "Unauthorized - You can only view your own invoices", http.StatusForbidden)
		return
	}

		invoiceView := views.MapInvoicePage(inv, items, "view")
	
	// Check if the invoice owner has Stripe Connect enabled.
	// Only show Pay Now to non-owners (clients viewing via access token).
	// The owner uses "Mark as Paid" instead.
	payNowEnabled := false
	user := auth.GetUser(r)
	isOwner := user != nil && inv.UserID != nil && user.ID == *inv.UserID
	if !isOwner && inv.UserID != nil && inv.Status != "paid" && inv.Status != "void" {
		owner, err := h.App.UserRepo.GetByID(*inv.UserID)
		if err == nil && owner.StripeConnectID != "" {
			payNowEnabled = true
		}
	}
		
	accessToken := r.URL.Query().Get("access")

	// Determine if branding footer should show (free tier only)
	showBranding := true
	if inv.UserID != nil {
		if owner, err := h.App.UserRepo.GetByID(*inv.UserID); err == nil && owner != nil {
			if owner.Plan == "pro" || owner.Plan == "growth" {
				showBranding = false
			}
		}
	}

	h.App.Render(w, r, invoiceTemplateName(invoiceView.TemplateID), map[string]any{
		"Invoice":       invoiceView,
		"IsLoggedIn":    auth.GetUser(r) != nil,
		"Sent":          r.URL.Query().Get("sent") == "true",
		"SentTo":        r.URL.Query().Get("to"),
		"Paid":          r.URL.Query().Get("paid") == "1",
		"PayNowEnabled": payNowEnabled,
		"AccessToken":   accessToken,
		"ShowBranding":  showBranding,
	})
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

	// PDF mode: headless Chromium cannot reliably fetch external images.
	// Fetch the logo here in Go and inject it as a base64 data URI instead.
	logoStr := string(invoiceView.LogoURL)
	if logoStr != "" {
		if strings.HasPrefix(logoStr, "/static/") {
			// Local disk (LocalStore)
			filePath := "." + logoStr
			if b, err := os.ReadFile(filePath); err == nil {
				mime := http.DetectContentType(b)
				invoiceView.LogoURL = template.URL("data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b))
			} else {
				log.Printf("[pdf] logo read from disk error for invoice %d: %v", id, err)
			}
		} else if strings.HasPrefix(logoStr, "http") {
			// Remote (SupabaseStore) — fetch and inline so Chrome needs no outbound network.
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
	var buf bytes.Buffer

	// Determine if branding footer should show (free tier only)
	showBranding := true
	if inv.UserID != nil {
		if owner, err := h.App.UserRepo.GetByID(*inv.UserID); err == nil && owner != nil {
			if owner.Plan == "pro" || owner.Plan == "growth" {
				showBranding = false
			}
		}
	}

	templateData := map[string]any{
		"Invoice":      invoiceView,
		"User":         user,
		"csrfField":    "",
		"ShowBranding": showBranding,
	}

		if err := h.App.Templates.ExecuteTemplate(&buf, invoiceTemplateName(invoiceView.TemplateID), templateData); err != nil {
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

func (h *Handlers) InvoicesList(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	invoices, err := h.App.InvRepo.ListInvoices(r.Context(), 50, 0, &user.ID)
	if err != nil {
		http.Error(w, "Failed to load invoices", http.StatusInternalServerError)
		return
	}

	monthlyCount, _ := h.App.UserRepo.GetMonthlyInvoiceCount(r.Context(), user.ID)

	h.App.Render(w, r, "invoices_list.tmpl", map[string]any{
		"Invoices":     invoices,
		"User":         user,
		"MonthlyCount": monthlyCount,
		"MonthlyLimit": freePlanMonthlyLimit,
		"IsPro":        user.Plan == "pro",
		"Deleted":      r.URL.Query().Get("deleted") == "true",
	})
}

func (h *Handlers) InvoiceEditGet(w http.ResponseWriter, r *http.Request) {
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

	invoiceView := views.MapInvoicePage(inv, items, "edit")

	// Pull logo from business profile if not already on invoice
	if invoiceView.LogoURL == "" {
		if bp, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID); err == nil && bp != nil {
			invoiceView.LogoURL = template.URL(bp.LogoURL)

		}
	}

	h.App.Render(w, r, "invoice_new.tmpl", map[string]any{
		"Invoice":    invoiceView,
		"IsEdit":     true,
		"User":       user,
		"USStates":   catalog.USStates,
		"Currencies": catalog.SupportedCurrencies,
		"Templates":  catalog.InvoiceTemplates,
	})
}

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

	// ── Parse line items ──────────────────────────────────────────────
	descriptions := r.Form["description[]"]
	details := r.Form["details[]"]
	quantities := r.Form["quantity[]"]
	unitPrices := r.Form["unit_price[]"]

	var items []repo.InvoiceItem
	for i, desc := range descriptions {
		desc = strings.TrimSpace(desc)
		if desc == "" {
			continue
		}
		qty := float64(1)
		if i < len(quantities) {
			qty, _ = strconv.ParseFloat(quantities[i], 64)
		}
		if qty <= 0 {
			qty = 1
		}
		var unitPrice float64
		if i < len(unitPrices) {
			unitPrice, _ = strconv.ParseFloat(unitPrices[i], 64)
		}
		detail := ""
		if i < len(details) {
			detail = strings.TrimSpace(details[i])
		}
		unitPriceCents := int64(math.Round(unitPrice * 100))
		items = append(items, repo.InvoiceItem{
			Description:    desc,
			Details:        detail,
			Quantity:       qty,
			UnitPriceCents: unitPriceCents,
		})
	}

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
	normalizeTemplateFields(inv, isPro)

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

		// ── Cancel pending reminders when invoice is paid or voided ──────
	if newStatus == "paid" || newStatus == "void" {
		cancelled, err := h.App.SchedulerRepo.CancelJobsForInvoice(r.Context(), id)
		if err != nil {
			log.Printf("[status] failed to cancel reminders for invoice %d: %v", id, err)
		} else if cancelled > 0 {
			log.Printf("[status] cancelled %d pending reminders for invoice %d", cancelled, id)
		}
	}

	// ── Schedule reminders when invoice is marked sent ────────────────
	// Requires: auto_reminders enabled on the invoice + a due date set.
	// If no due date, we have nothing to anchor reminders to — skip silently.
	if newStatus == "sent" && inv.AutoReminders && inv.DueDate != nil {
		// Cancel any stale reminders first (e.g. re-sending a previously sent invoice)
		_, _ = h.App.SchedulerRepo.CancelJobsForInvoice(r.Context(), id)

		reminderSchedule := []struct {
			offset       time.Duration
			reminderType string
		}{
			{-3 * 24 * time.Hour, "due_soon"},
			{0, "due_today"},
			{3 * 24 * time.Hour, "overdue"},
			{7 * 24 * time.Hour, "overdue"},
			{14 * 24 * time.Hour, "overdue"},
		}

		for _, rem := range reminderSchedule {
			runAt := inv.DueDate.Add(rem.offset)
			if runAt.Before(time.Now()) {
				// Don't schedule reminders that are already in the past
				continue
			}
			payload := map[string]any{
				"invoice_id":    id,
				"reminder_type": rem.reminderType,
			}
			_, err := h.App.SchedulerRepo.CreateJob(r.Context(), "send_reminder", payload, runAt)
			if err != nil {
				log.Printf("[status] failed to schedule %s reminder for invoice %d: %v",
					rem.reminderType, id, err)
			} else {
				log.Printf("[status] scheduled %s reminder for invoice %d at %s",
					rem.reminderType, id, runAt.Format("2006-01-02 15:04"))
			}
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
// invoiceTemplateName returns the detail template filename for the given
// template ID. Falls back to classic if the template file would not exist.
func invoiceTemplateName(templateID string) string {
	switch templateID {
	case "minimal", "bold":
		return "invoice_" + templateID + ".tmpl"
	default:
		return "invoice_detail.tmpl"
	}
}

// calculateNextRun returns the next run time based on frequency.
func calculateNextRun(frequency string, from time.Time) time.Time {
	switch frequency {
	case "weekly":
		return from.AddDate(0, 0, 7)
	case "monthly":
		return from.AddDate(0, 1, 0)
	case "quarterly":
		return from.AddDate(0, 3, 0)
	case "yearly":
		return from.AddDate(1, 0, 0)
	default:
		return from.AddDate(0, 1, 0)
	}
}

// isValidHexColor returns true if s is a valid #RRGGBB hex color.
func isValidHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
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

// normalizeTemplateFields validates and applies Pro gating to
// template_id and brand_color. Free users are silently reset
// to defaults regardless of what the form submitted.
func normalizeTemplateFields(inv *repo.Invoice, isPro bool) {
	if !catalog.ValidTemplateID(inv.TemplateID) {
		inv.TemplateID = catalog.DefaultTemplateID
	}
	if !isValidHexColor(inv.BrandColor) {
		inv.BrandColor = catalog.DefaultBrandColor
	}
	if !isPro {
		inv.TemplateID = catalog.DefaultTemplateID
		inv.BrandColor = catalog.DefaultBrandColor
	}
}
