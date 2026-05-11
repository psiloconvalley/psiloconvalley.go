// internal/handlers/invoice.go
package handlers

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/forms"
	"psiloconvalley/internal/pdf"
	"psiloconvalley/internal/repo"
	"psiloconvalley/internal/views"
)

func (h *Handlers) InvoicesList(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	var uid *int64
	if user != nil {
		uid = &user.ID
	}

	list, err := h.App.InvRepo.ListInvoices(r.Context(), 50, 0, uid)
	if err != nil {
		log.Printf("list invoices error: %v", err)
		http.Error(w, "Could not load invoices", http.StatusInternalServerError)
		return
	}

	rows := views.MapInvoiceList(list)
	h.App.Render(w, r, "invoices_list.tmpl", map[string]any{"Invoices": rows})
}

func (h *Handlers) InvoiceDetail(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || !h.canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	invoiceView := views.MapInvoicePage(inv, items, "view")
	h.App.Render(w, r, "invoice_detail.tmpl", map[string]any{"Invoice": invoiceView})
}

func (h *Handlers) InvoiceNewGet(w http.ResponseWriter, r *http.Request) {
	if h.hasReachedLimit(r) {
		http.Redirect(w, r, "/register?reason=limit", http.StatusSeeOther)
		return
	}

	inv := &repo.Invoice{
		CompanyCountry: catalog.DefaultCompanyCountry,
		CompanyState:   catalog.DefaultCompanyState,
		ClientCountry:  catalog.DefaultClientCountry,
		ClientState:    catalog.DefaultClientState,
		Currency:       catalog.DefaultCurrency,
		Status:         "draft",
		IssueDate:      time.Now(),
	}

	user := auth.GetUser(r)
	var clients []repo.Client

	if user != nil {
		profile, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID)
		if err == nil && profile != nil {
			inv.CompanyName = profile.Name
			inv.CompanyEmail = profile.Email
			inv.CompanyAddress = profile.Address
			inv.CompanyCity = profile.City
			inv.CompanyState = profile.State
			inv.CompanyZip = profile.Zip
			inv.CompanyCountry = profile.Country
			if profile.Currency != "" {
				inv.Currency = profile.Currency
			}
		}

		clients, err = h.App.ClientRepo.ListByUserID(r.Context(), user.ID)
		if err != nil {
			log.Printf("CLIENT LOAD ERROR for user %d: %v", user.ID, err)
		}
	}

	invoiceView := views.MapInvoicePage(inv, nil, "create")
	h.App.Render(w, r, "invoice_new.tmpl", map[string]any{
		"Invoice":    invoiceView,
		"Mode":       "create",
		"Currencies": catalog.SupportedCurrencies,
		"Clients":    clients,
	})
}

func (h *Handlers) InvoiceCreatePost(w http.ResponseWriter, r *http.Request) {
	if h.hasReachedLimit(r) {
		http.Redirect(w, r, "/register?reason=limit", http.StatusSeeOther)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	f, err := forms.ParseInvoiceForm(r)
	if err != nil {
		log.Printf("parseInvoiceForm error: %v", err)
		http.Error(w, "Bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	if f.InvoiceNumber == "" {
		f.InvoiceNumber = fmt.Sprintf("INV-%d", time.Now().UnixNano()/1_000_000)
	}

	user := auth.GetUser(r)
	var uid *int64
	if user != nil {
		uid = &user.ID
	}

	if user != nil && f.ClientName != "" && f.ClientID == nil {
		h.autoCreateClient(r, user, f)
	}

	inv := formToInvoice(f, uid, "draft")

	id, err := h.App.InvRepo.CreateInvoice(r.Context(), inv, f.Items)
	if err != nil {
		log.Printf("create invoice error: %v", err)
		http.Error(w, "Save failed", http.StatusInternalServerError)
		return
	}

	if user == nil {
		auth.SetAnonInvoiceCount(w, auth.GetAnonInvoiceCount(r)+1)
	}

	http.Redirect(w, r, fmt.Sprintf("/invoices/%d", id), http.StatusSeeOther)
}

// =====================================================================
// MISSING HANDLERS ADDED BELOW (Edit & Duplicate)
// =====================================================================

func (h *Handlers) InvoiceEditGet(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || !h.canAccessInvoice(r, inv) || inv.Status != "draft" {
		http.NotFound(w, r)
		return
	}

	user := auth.GetUser(r)
	var clients []repo.Client
	if user != nil {
		clients, _ = h.App.ClientRepo.ListByUserID(r.Context(), user.ID)
	}

	invoiceView := views.MapInvoicePage(inv, items, "edit")
	h.App.Render(w, r, "invoice_new.tmpl", map[string]any{
		"Invoice":    invoiceView,
		"Mode":       "edit",
		"Currencies": catalog.SupportedCurrencies,
		"Clients":    clients,
	})
}

func (h *Handlers) InvoiceUpdatePost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	existing, _, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || !h.canAccessInvoice(r, existing) {
		http.NotFound(w, r)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	f, err := forms.ParseInvoiceForm(r)
	if err != nil {
		log.Printf("parseInvoiceForm error: %v", err)
		http.Error(w, "Bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	user := auth.GetUser(r)
	if user != nil && f.ClientName != "" && f.ClientID == nil {
		h.autoCreateClient(r, user, f)
	}

	inv := formToInvoice(f, existing.UserID, existing.Status)
	inv.ID = id
	inv.InvoiceNumber = existing.InvoiceNumber

	if err := h.App.InvRepo.UpdateInvoice(r.Context(), inv, f.Items); err != nil {
		log.Printf("update invoice error: %v", err)
		http.Error(w, "Update failed", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/invoices/%d", id), http.StatusSeeOther)
}

func (h *Handlers) InvoiceDuplicateGet(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || !h.canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	inv.ID = 0
	inv.InvoiceNumber = ""
	inv.IssueDate = time.Now()
	inv.Status = "draft"

	user := auth.GetUser(r)
	var clients []repo.Client
	if user != nil {
		clients, _ = h.App.ClientRepo.ListByUserID(r.Context(), user.ID)
	}

	invoiceView := views.MapInvoicePage(inv, items, "duplicate")
	h.App.Render(w, r, "invoice_new.tmpl", map[string]any{
		"Invoice":    invoiceView,
		"Mode":       "duplicate",
		"Currencies": catalog.SupportedCurrencies,
		"Clients":    clients,
	})
}

func (h *Handlers) InvoiceStatusPost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	newStatus := r.FormValue("status")

	if err := h.App.InvRepo.UpdateInvoiceStatus(r.Context(), id, newStatus, user.ID); err != nil {
		log.Printf("status update error: %v", err)
		http.Error(w, "Could not update status", http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/invoices/%d", id), http.StatusSeeOther)
}

func (h *Handlers) InvoicePDFGet(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || !h.canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	invoiceView := views.MapInvoicePage(inv, items, "view")

	var buf bytes.Buffer
	data := map[string]any{
		"Invoice":   invoiceView,
		"User":      auth.GetUser(r),
		"csrfField": "",
	}

	if err := h.App.Templates.ExecuteTemplate(&buf, "invoice_detail.tmpl", data); err != nil {
		log.Printf("pdf template error: %v", err)
		http.Error(w, "Could not render invoice", http.StatusInternalServerError)
		return
	}

	pdfBytes, err := pdf.Generate(r.Context(), buf.String())
	if err != nil {
		log.Printf("pdf generation error: %v", err)
		http.Error(w, "Could not generate PDF", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("invoice-%s.pdf", inv.InvoiceNumber)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

// =====================================================================
// PRIVATE HELPERS
// =====================================================================

func (h *Handlers) autoCreateClient(r *http.Request, user *repo.User, f *forms.InvoiceFormData) {
	profile, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID)
	if err != nil || profile == nil {
		return
	}
	if !h.canAddClient(r) {
		return
	}

	clientID, err := h.App.ClientRepo.FindOrCreate(
		r.Context(), profile.ID,
		f.ClientName, f.ClientEmail, f.ClientAddress,
		f.ClientCity, f.ClientState, f.ClientZip, f.ClientCountry,
	)
	if err != nil {
		log.Printf("auto-create client error: %v", err)
		return
	}

	f.ClientID = &clientID
	log.Printf("AUTO-SAVED CLIENT: %s (id=%d)", f.ClientName, clientID)
}

func formToInvoice(f *forms.InvoiceFormData, userID *int64, status string) *repo.Invoice {
	return &repo.Invoice{
		UserID:         userID,
		ClientID:       f.ClientID,
		CompanyName:    f.CompanyName,
		CompanyEmail:   f.CompanyEmail,
		CompanyAddress: f.CompanyAddress,
		CompanyCity:    f.CompanyCity,
		CompanyZip:     f.CompanyZip,
		CompanyState:   f.CompanyState,
		CompanyCountry: f.CompanyCountry,
		ClientName:     f.ClientName,
		ClientEmail:    f.ClientEmail,
		ClientAddress:  f.ClientAddress,
		ClientCity:     f.ClientCity,
		ClientZip:      f.ClientZip,
		ClientState:    f.ClientState,
		ClientCountry:  f.ClientCountry,
		InvoiceNumber:  f.InvoiceNumber,
		IssueDate:      f.IssueDate,
		DueDate:        f.DueDate,
		TaxRateBps:     f.TaxRateBps,
		Notes:          f.Notes,
		PaymentDetails: f.PaymentDetails,
		Currency:       f.Currency,
		Status:         status,
	}
}
