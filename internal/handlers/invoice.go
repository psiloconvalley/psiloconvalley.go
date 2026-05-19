// internal/handlers/invoice.go
package handlers

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
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


}
if user != nil {
    if bp, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID); err == nil && bp != nil {
        invoiceData.LogoURL = bp.LogoURL
    }
}
data := map[string]any{
    "User":       user,
    "IsLoggedIn": user != nil,
    "Invoice":    invoiceData,
    "Mode":       "create",
    "Currencies": catalog.SupportedCurrencies,
    "USStates":   catalog.USStates,
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

	if len(errs) > 0 {
		data := map[string]any{
			"User":       user,
			"IsLoggedIn": user != nil,
			"Mode":       "create",
			"Currencies": catalog.SupportedCurrencies,
			"USStates":   catalog.USStates,
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
				AutoReminders:  r.FormValue("auto_reminders") == "on",
				LogoURL: func() string {
					if user != nil {
						if bp, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID); err == nil && bp != nil {
							return bp.LogoURL
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
	autoReminders :=  r.FormValue("auto_reminders") == "on"


	// ── Auto-generate invoice number if blank ─────────────────────────
	if invoiceNumber == "" {
		invoiceNumber = fmt.Sprintf("INV-%d", time.Now().UnixNano())
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
		UserID:            userID,
		BusinessProfileID: bizProfileID,
		AnonymousToken:    anonymousToken,
		CompanyName:       companyName,
		CompanyEmail:      strings.TrimSpace(r.FormValue("company_email")),
		CompanyAddress:    strings.TrimSpace(r.FormValue("company_address")),
		CompanyCity:       strings.TrimSpace(r.FormValue("company_city")),
		CompanyZip:        strings.TrimSpace(r.FormValue("company_zip")),
		CompanyState:      strings.TrimSpace(r.FormValue("company_state")),
		CompanyCountry:    strings.TrimSpace(r.FormValue("company_country")),
		ClientName:        clientName,
		ClientEmail:       strings.TrimSpace(r.FormValue("client_email")),
		ClientAddress:     strings.TrimSpace(r.FormValue("client_address")),
		ClientCity:        strings.TrimSpace(r.FormValue("client_city")),
		ClientZip:         strings.TrimSpace(r.FormValue("client_zip")),
		ClientState:       strings.TrimSpace(r.FormValue("client_state")),
		ClientCountry:     strings.TrimSpace(r.FormValue("client_country")),
		InvoiceNumber:     invoiceNumber,
		IssueDate:         issueDate,
		DueDate:           dueDate,
		TaxRateBps:        taxRateBps,
		DiscountAmountCents: discountCents,
		ShowLogo:          showLogo,
		ShowTitle:         showTitle,
		AutoReminders:     autoReminders,
		Currency:          currency,
		Notes:             strings.TrimSpace(r.FormValue("notes")),
		PaymentDetails:    strings.TrimSpace(r.FormValue("payment_details")),
		Status:            "draft",
	}


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
					LogoURL: func() string {
						if user != nil {
							if bp, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID); err == nil && bp != nil {
								return bp.LogoURL
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

	if !h.canAccessInvoice(r, inv) {
		http.Error(w, "Unauthorized - You can only view your own invoices", http.StatusForbidden)
		return
	}

	invoiceView := views.MapInvoicePage(inv, items, "view")

	h.App.Render(w, r, "invoice_detail.tmpl", map[string]any{
		"Invoice":    invoiceView,
		"IsLoggedIn": auth.GetUser(r) != nil,
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

	var buf bytes.Buffer

	templateData := map[string]any{
		"Invoice":   invoiceView,
		"User":      user,
		"csrfField": "",
	}

	if err := h.App.Templates.ExecuteTemplate(&buf, "invoice_detail.tmpl", templateData); err != nil {
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
			invoiceView.LogoURL = bp.LogoURL
		}
	}

	h.App.Render(w, r, "invoice_new.tmpl", map[string]any{
		"Invoice":    invoiceView,
		"IsEdit":     true,
		"User":       user,
		"USStates":   catalog.USStates,
		"Currencies": catalog.SupportedCurrencies,
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
	details      := r.Form["details[]"]
	quantities   := r.Form["quantity[]"]
	unitPrices   := r.Form["unit_price[]"]

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
	inv.CompanyName    = strings.TrimSpace(r.FormValue("company_name"))
	inv.CompanyEmail   = strings.TrimSpace(r.FormValue("company_email"))
	inv.CompanyAddress = strings.TrimSpace(r.FormValue("company_address"))
	inv.CompanyCity    = strings.TrimSpace(r.FormValue("company_city"))
	inv.CompanyZip     = strings.TrimSpace(r.FormValue("company_zip"))
	inv.CompanyState   = strings.TrimSpace(r.FormValue("company_state"))
	inv.CompanyCountry = strings.TrimSpace(r.FormValue("company_country"))
	inv.ClientName     = strings.TrimSpace(r.FormValue("client_name"))
	inv.ClientEmail    = strings.TrimSpace(r.FormValue("client_email"))
	inv.ClientAddress  = strings.TrimSpace(r.FormValue("client_address"))
	inv.ClientCity     = strings.TrimSpace(r.FormValue("client_city"))
	inv.ClientZip      = strings.TrimSpace(r.FormValue("client_zip"))
	inv.ClientState    = strings.TrimSpace(r.FormValue("client_state"))
	inv.ClientCountry  = strings.TrimSpace(r.FormValue("client_country"))
	inv.ShowLogo  = r.FormValue("show_logo") == "on"
	inv.ShowTitle = r.FormValue("show_title") == "on"
	inv.AutoReminders = r.FormValue("auto_reminders") == "on"
	inv.Currency            = currency
	inv.TaxRateBps          = taxRateBps
	inv.DiscountAmountCents = discountCents
	inv.Notes          = strings.TrimSpace(r.FormValue("notes"))
	inv.PaymentDetails = strings.TrimSpace(r.FormValue("payment_details"))

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
