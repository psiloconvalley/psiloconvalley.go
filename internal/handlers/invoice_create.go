// internal/handlers/invoice_create.go
package handlers

import (
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/repo"
	"psiloconvalley/internal/service"
	"psiloconvalley/internal/views"
)

// InvoiceCreatePost handles invoice creation for both anonymous and
// authenticated users. Anonymous users are tracked via a signed cookie
// and limited to the freemium cap before being prompted to register.
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

	// ── Parse form fields ────────────────────────────────────────────
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
		errs = append(errs, FormError{
			Field:   "client_name",
			Message: "Customer name is required (at least 2 characters)",
		})
	}
	if companyName == "" {
		errs = append(errs, FormError{
			Field:   "company_name",
			Message: "Company name is required",
		})
	}
	if len(items) == 0 {
		errs = append(errs, FormError{
			Field:   "items",
			Message: "At least one line item with a description is required",
		})
	}
	if invoiceNumber != "" && user != nil {
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
			"User":         user,
			"IsLoggedIn":   user != nil,
			"Mode":         "create",
			"DocumentType": "invoice",
			"Currencies":   catalog.SupportedCurrencies,
			"USStates":     catalog.USStates,
			"Templates":    catalog.InvoiceTemplates,
			"Errors":       errs,
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

	// ── Numeric fields ───────────────────────────────────────────────
	taxRatePct, _ := strconv.ParseFloat(r.FormValue("tax_rate"), 64)
	taxRateBps := int64(math.Round(taxRatePct * 100))
	discountAmt, _ := strconv.ParseFloat(r.FormValue("discount_amount"), 64)
	discountCents := int64(math.Round(discountAmt * 100))
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

	// ── Build invoice struct ──────────────────────────────────────────
	var userID *int64
	var bizProfileID *int64
	if user != nil {
		userID = &user.ID
		if bp, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID); err == nil {
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

	service.NormalizeTemplateFields(inv, user != nil && user.Plan == "pro")

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
				{
					Field:   "invoice_number",
					Message: "Invoice number already exists. Please choose a different one or leave it blank to auto-generate.",
				},
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
		if err := h.App.InvService.CreateRecurringSchedule(r.Context(), service.ScheduleRecurringParams{
			UserID:    user.ID,
			InvoiceID: invoiceID,
			Frequency: r.FormValue("recurring_frequency"),
			AutoSend:  r.FormValue("recurring_auto_send") == "on",
		}); err != nil {
			log.Printf("[invoice] failed to create recurring schedule: %v", err)
		}
	}

	http.Redirect(w, r, "/invoices/"+strconv.FormatInt(invoiceID, 10), http.StatusSeeOther)
}
