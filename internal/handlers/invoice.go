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

// =====================================================================
// Private: ID extraction
//
// FIX C3: Every handler previously discarded the ParseInt error with _.
// A malformed URL like /invoices/abc/edit would produce id=0, then
// GetInvoiceWithItems(ctx, 0) would either return sql.ErrNoRows or —
// worse — match a real record with id=0 if one exists. Both outcomes
// are wrong. Now any non-numeric or non-positive ID is an explicit 404.
// =====================================================================

func invoiceIDFromURL(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return 0, false
	}
	return id, true
}

// =====================================================================
// List
// =====================================================================

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
	h.App.Render(w, r, "invoices_list.tmpl", map[string]any{
		"Invoices": rows,
	})
}

// =====================================================================
// Detail (view only)
// =====================================================================

func (h *Handlers) InvoiceDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := invoiceIDFromURL(w, r)
	if !ok {
		return
	}

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || !h.canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	if inv.UserID != nil {
		if profile, err := h.App.BizRepo.GetByUserID(r.Context(), *inv.UserID); err == nil && profile != nil {
			inv.LogoURL = profile.LogoURL
		}
	}

	invoiceView := views.MapInvoicePage(inv, items, "view")
	h.App.Render(w, r, "invoice_detail.tmpl", map[string]any{
		"Invoice": invoiceView,
	})
}

// =====================================================================
// New (GET)
// =====================================================================

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
			inv.CompanyName    = profile.Name
			inv.CompanyEmail   = profile.Email
			inv.CompanyAddress = profile.Address
			inv.CompanyCity    = profile.City
			inv.CompanyState   = profile.State
			inv.CompanyZip     = profile.Zip
			inv.CompanyCountry = profile.Country
			if profile.Currency != "" {
				inv.Currency = profile.Currency
			}
		}

		clients, err = h.App.ClientRepo.ListByUserID(r.Context(), user.ID)
		if err != nil {
			log.Printf("client load error for user %d: %v", user.ID, err)
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

// =====================================================================
// Create (POST)
//
// FIX M2: On validation failure, re-render the form with errors and
// all previously-entered data intact. The user never loses their work.
// =====================================================================

func (h *Handlers) InvoiceCreatePost(w http.ResponseWriter, r *http.Request) {
	if h.hasReachedLimit(r) {
		http.Redirect(w, r, "/register?reason=limit", http.StatusSeeOther)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	result, err := forms.ParseInvoiceForm(r)
	if err != nil {
		// True HTTP-level error (body too large, malformed multipart).
		// Not a validation error — no form data to preserve.
		log.Printf("parseInvoiceForm http error: %v", err)
		http.Error(w, "Bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	if !result.Valid() {
		// FIX M2: validation failure — re-render form with errors inline.
		h.reRenderInvoiceForm(w, r, result, "create", nil)
		return
	}

	f := result.Data

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
// Edit (GET)
// =====================================================================

func (h *Handlers) InvoiceEditGet(w http.ResponseWriter, r *http.Request) {
	id, ok := invoiceIDFromURL(w, r)
	if !ok {
		return
	}

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	// FIX C4: draft-only guard is enforced on GET. The POST handler below
	// mirrors this check. Both must agree — a GET-only guard is bypassable
	// via direct POST to /{id}/edit.
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

// =====================================================================
// Update (POST)
//
// FIX C4: draft-only guard added — mirrors InvoiceEditGet.
// FIX M2: validation failures re-render with data intact.
// =====================================================================

func (h *Handlers) InvoiceUpdatePost(w http.ResponseWriter, r *http.Request) {
	id, ok := invoiceIDFromURL(w, r)
	if !ok {
		return
	}

	existing, existingItems, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	// FIX C4: status guard added here — previously only on GET.
	// A direct POST to /{id}/edit on a sent/paid invoice was not blocked.
	if err != nil || !h.canAccessInvoice(r, existing) || existing.Status != "draft" {
		http.NotFound(w, r)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	result, err := forms.ParseInvoiceForm(r)
	if err != nil {
		log.Printf("parseInvoiceForm http error: %v", err)
		http.Error(w, "Bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	if !result.Valid() {
		// Re-render the edit form with errors and existing item data preserved.
		h.reRenderInvoiceForm(w, r, result, "edit", existingItems)
		return
	}

	f := result.Data

	user := auth.GetUser(r)
	if user != nil && f.ClientName != "" && f.ClientID == nil {
		h.autoCreateClient(r, user, f)
	}

	inv := formToInvoice(f, existing.UserID, existing.Status)
	inv.ID            = id
	inv.InvoiceNumber = existing.InvoiceNumber

	if err := h.App.InvRepo.UpdateInvoice(r.Context(), inv, f.Items); err != nil {
		log.Printf("update invoice error: %v", err)
		http.Error(w, "Update failed", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/invoices/%d", id), http.StatusSeeOther)
}

// =====================================================================
// Duplicate (GET)
// FIX H2: this handler now exists AND is registered in router.go.
// =====================================================================

func (h *Handlers) InvoiceDuplicateGet(w http.ResponseWriter, r *http.Request) {
	id, ok := invoiceIDFromURL(w, r)
	if !ok {
		return
	}

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || !h.canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	// Reset identity fields — this becomes a new draft.
	inv.ID            = 0
	inv.InvoiceNumber = ""
	inv.IssueDate     = time.Now()
	inv.Status        = "draft"

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

// =====================================================================
// Status Update (POST)
// =====================================================================

func (h *Handlers) InvoiceStatusPost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	id, ok := invoiceIDFromURL(w, r)
	if !ok {
		return
	}

	// H1 note: the allowlist validation now lives in repo.UpdateInvoiceStatus.
	// We rely on it there (single source of truth for business rules) and
	// surface its error message directly — no duplicate validation needed here.
	newStatus := r.FormValue("status")
	if newStatus == "" {
		http.Error(w, "Status is required", http.StatusBadRequest)
		return
	}

	if err := h.App.InvRepo.UpdateInvoiceStatus(r.Context(), id, newStatus, user.ID); err != nil {
		log.Printf("status update error: %v", err)
		http.Error(w, "Could not update status: "+err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/invoices/%d", id), http.StatusSeeOther)
}

// =====================================================================
// PDF (GET)
//
// FIX M1: Content-Disposition is now driven by the ?mode= query param.
//
//   /invoices/42/pdf            → attachment (download) — desktop default
//   /invoices/42/pdf?mode=inline → inline (browser preview) — mobile link
//
// Why query param instead of user-agent sniffing?
//   1. UA strings change constantly and are unreliable on embedded browsers.
//   2. Query params are explicit, testable, and cache-key-friendly.
//   3. The mobile template simply uses ?mode=inline for its "Preview" button;
//      the desktop template omits it. Zero handler duplication.
// =====================================================================

func (h *Handlers) InvoicePDFGet(w http.ResponseWriter, r *http.Request) {
	id, ok := invoiceIDFromURL(w, r)
	if !ok {
		return
	}

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || !h.canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	invoiceView := views.MapInvoicePage(inv, items, "view")

	// Signal to the template that we are rendering for PDF:
	// suppress navigation, action buttons, and interactive elements.
	invoiceView.Hints.PDFMode = true

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

	// FIX M1: choose inline vs attachment based on explicit query param.
	disposition := fmt.Sprintf(`attachment; filename="invoice-%s.pdf"`, inv.InvoiceNumber)
	if r.URL.Query().Get("mode") == "inline" {
		disposition = fmt.Sprintf(`inline; filename="invoice-%s.pdf"`, inv.InvoiceNumber)
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
	_, _ = w.Write(pdfBytes)
	// Note: no explicit w.WriteHeader(200) — Write calls it implicitly.
	// Explicit WriteHeader before Write is correct; after headers are set
	// but before Write is redundant and misleading to future readers.
}

// =====================================================================
// Private Helpers
// =====================================================================

// reRenderInvoiceForm re-renders invoice_new.tmpl with validation errors
// and the user's previously entered data. Used by both Create and Update
// POST handlers on validation failure.
//
// existingItems is non-nil only in the edit path — it provides fallback
// item data when the submitted items fail validation entirely.
func (h *Handlers) reRenderInvoiceForm(
	w http.ResponseWriter,
	r *http.Request,
	result *forms.ParseResult,
	mode string,
	existingItems []repo.InvoiceItem,
) {
	user := auth.GetUser(r)
	var clients []repo.Client
	if user != nil {
		clients, _ = h.App.ClientRepo.ListByUserID(r.Context(), user.ID)
	}

	// Build a partial InvoicePage from the submitted (invalid) data
	// so the template can repopulate every field correctly.
	f := result.Data
	partialInv := &repo.Invoice{
		CompanyName:    f.CompanyName,
		CompanyEmail:   f.CompanyEmail,
		CompanyAddress: f.CompanyAddress,
		CompanyCity:    f.CompanyCity,
		CompanyState:   f.CompanyState,
		CompanyZip:     f.CompanyZip,
		CompanyCountry: f.CompanyCountry,
		ClientName:     f.ClientName,
		ClientEmail:    f.ClientEmail,
		ClientAddress:  f.ClientAddress,
		ClientCity:     f.ClientCity,
		ClientState:    f.ClientState,
		ClientZip:      f.ClientZip,
		ClientCountry:  f.ClientCountry,
		InvoiceNumber:  f.InvoiceNumber,
		IssueDate:      f.IssueDate,
		DueDate:        f.DueDate,
		TaxRateBps:     f.TaxRateBps,
		Notes:          f.Notes,
		PaymentDetails: f.PaymentDetails,
		Currency:       f.Currency,
		Status:         "draft",
	}

	// Use submitted items if any parsed successfully; otherwise fall back
	// to the existing saved items (edit path only).
	itemsToShow := f.Items
	if len(itemsToShow) == 0 && len(existingItems) > 0 {
		itemsToShow = existingItems
	}

	invoiceView := views.MapInvoicePage(partialInv, itemsToShow, mode)

	h.App.Render(w, r, "invoice_new.tmpl", map[string]any{
		"Invoice":    invoiceView,
		"Mode":       mode,
		"Currencies": catalog.SupportedCurrencies,
		"Clients":    clients,
		"Errors":     result.Errors, // template renders these inline
	})
}

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
	log.Printf("auto-saved client: %s (id=%d)", f.ClientName, clientID)
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
