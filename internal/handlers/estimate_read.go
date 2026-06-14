// internal/handlers/estimate_read.go
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

// EstimatesList renders the paginated estimate list for the logged-in user.
func (h *Handlers) EstimatesList(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	estimates, err := h.App.InvRepo.ListEstimates(r.Context(), 50, 0, user.ID)
	if err != nil {
		http.Error(w, "Failed to load estimates", http.StatusInternalServerError)
		return
	}

	h.App.Render(w, r, "estimates_list.tmpl", map[string]any{
		"Estimates": estimates,
		"User":      user,
		"IsPro":     user.Plan == "pro",
		"Deleted":   r.URL.Query().Get("deleted") == "true",
		"Converted": r.URL.Query().Get("converted") == "true",
	})
}

// EstimateNewGet renders the blank estimate creation form.
func (h *Handlers) EstimateNewGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)

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
		"DocumentType": "estimate",
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

// EstimateDetail renders the read-only estimate view page.
func (h *Handlers) EstimateDetail(w http.ResponseWriter, r *http.Request) {
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

	if inv.DocumentType != "estimate" {
		http.NotFound(w, r)
		return
	}

	if !h.canViewInvoice(r, inv) {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	invoiceView := views.MapInvoicePage(inv, items, "view")
	accessToken := r.URL.Query().Get("access")

	h.App.Render(w, r, service.InvoiceTemplateName(invoiceView.TemplateID), map[string]any{
		"Invoice":      invoiceView,
		"IsLoggedIn":   auth.GetUser(r) != nil,
		"IsEstimate":   true,
		"DocumentType": "estimate",
		"AccessToken":  accessToken,
		"Converted":    r.URL.Query().Get("converted") == "true",
		"Sent":         r.URL.Query().Get("sent") == "true",
		"SentTo":       r.URL.Query().Get("to"),
	})
}
// EstimateEditGet renders the estimate edit form pre-populated with existing data.
func (h *Handlers) EstimateEditGet(w http.ResponseWriter, r *http.Request) {
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

	if inv.DocumentType != "estimate" {
		http.NotFound(w, r)
		return
	}

	// Revert accepted estimate to draft so client must re-accept after edits.
	if r.URL.Query().Get("revert") == "true" && inv.Status == "accepted" {
		if err := h.App.InvRepo.UpdateEstimateStatus(r.Context(), id, user.ID, "draft"); err != nil {
			http.Error(w, "Failed to revert estimate", http.StatusInternalServerError)
			return
		}
		inv.Status = "draft"
	}

	invoiceView := views.MapInvoicePage(inv, items, "edit")
	if invoiceView.LogoURL == "" {
		if bp, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID); err == nil && bp != nil {
			invoiceView.LogoURL = template.URL(bp.LogoURL)
		}
	}

	data := map[string]any{
		"Invoice":      invoiceView,
		"IsEdit":       true,
		"User":         user,
		"Mode":         "edit",
		"DocumentType": "estimate",
		"USStates":     catalog.USStates,
		"Currencies":   catalog.SupportedCurrencies,
		"Templates":    catalog.InvoiceTemplates,
	}

	clients, err := h.App.ClientRepo.ListByUserID(r.Context(), user.ID)
	if err == nil && len(clients) > 0 {
		data["Clients"] = clients
	}

	h.App.Render(w, r, "invoice_new.tmpl", data)
}

