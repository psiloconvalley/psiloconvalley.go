// internal/handlers/invoice_read.go
package handlers

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/service"
	"psiloconvalley/internal/views"
)

// InvoiceNewGet renders the blank invoice creation form.
// Anonymous users are allowed up to the anon limit.
// Logged-in users are checked against their plan limit.
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
			invoiceData.LogoURL = template.URL(bp.LogoURL)
			if bp.Name != "" {
				invoiceData.CompanyName = bp.Name
			}
			if bp.Email != "" {
				invoiceData.CompanyEmail = bp.Email
			}
			if bp.Address != "" {
				invoiceData.CompanyAddress = bp.Address
			}
			if bp.City != "" {
				invoiceData.CompanyCity = bp.City
			}
			if bp.State != "" {
				invoiceData.CompanyState = catalog.NormalizeState(bp.State)
			}
			if bp.Zip != "" {
				invoiceData.CompanyZip = bp.Zip
			}
			if bp.Country != "" {
				invoiceData.CompanyCountry = bp.Country
			}
		}
	}

	data := map[string]any{
		"User":         user,
		"IsLoggedIn":   user != nil,
		"Invoice":      invoiceData,
		"Mode":         "create",
		"DocumentType": "invoice",
		"Currencies":   catalog.SupportedCurrencies,
		"USStates":     catalog.USStates,
		"Templates":    catalog.InvoiceTemplates,
	}

	if user != nil {
		clients, err := h.App.ClientRepo.ListByUserID(r.Context(), user.ID)
		if err == nil && len(clients) > 0 {
			data["Clients"] = clients
		}
	}

	h.App.Render(w, r, "invoice_new.tmpl", data)
}

// InvoicesList renders the paginated invoice list for the logged-in user.
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

// InvoiceDetail renders the invoice view page.
// Read-only — anyone with the link or access token can view.
// Pay Now is shown only to non-owners when Stripe Connect is enabled.
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

	// Pay Now: only shown to non-owners when invoice owner has Stripe Connect.
	payNowEnabled := false
	user := auth.GetUser(r)
	isOwner := user != nil && inv.UserID != nil && user.ID == *inv.UserID
	if !isOwner && inv.UserID != nil && inv.Status != "paid" && inv.Status != "void" {
		owner, err := h.App.UserRepo.GetByID(*inv.UserID)
		if err == nil && owner.StripeConnectID != "" {
			payNowEnabled = true
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

	h.App.Render(w, r, service.InvoiceTemplateName(invoiceView.TemplateID), map[string]any{
		"Invoice":       invoiceView,
		"IsLoggedIn":    user != nil,
		"Sent":          r.URL.Query().Get("sent") == "true",
		"SentTo":        r.URL.Query().Get("to"),
		"Paid":          r.URL.Query().Get("paid") == "1",
		"PayNowEnabled": payNowEnabled,
		"AccessToken":   r.URL.Query().Get("access"),
		"ShowBranding":  showBranding,
	})
}

// InvoiceEditGet renders the invoice edit form pre-populated with existing data.
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

	// Pull logo from business profile if not already on the invoice.
	if invoiceView.LogoURL == "" {
		if bp, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID); err == nil && bp != nil {
			invoiceView.LogoURL = template.URL(bp.LogoURL)
		}
	}

	h.App.Render(w, r, "invoice_new.tmpl", map[string]any{
		"Invoice":      invoiceView,
		"IsEdit":       true,
		"Mode":         "edit",
		"DocumentType": inv.DocumentType,
		"User":         user,
		"USStates":     catalog.USStates,
		"Currencies":   catalog.SupportedCurrencies,
		"Templates":    catalog.InvoiceTemplates,
	})
}
