// internal/handlers/invoice.go
package handlers

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/pdf"
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

	h.App.Render(w, r, "invoice_new.tmpl", map[string]any{
		"User":       user,
		"IsLoggedIn": user != nil,
	})
}

func (h *Handlers) InvoiceCreatePost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	user := auth.GetUser(r)

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
				http.Error(w, "Failed to generate anonymous ownership token", http.StatusInternalServerError)
				return
			}

			anonymousToken = token
			auth.SetAnonymousToken(w, anonymousToken)
		}

		count := auth.GetAnonInvoiceCount(r)
		auth.SetAnonInvoiceCount(w, count+1)
	}

	amount, _ := strconv.ParseFloat(r.FormValue("amount"), 64)

	invoiceID, err := h.App.InvRepo.CreateWithToken(
		r.Context(),
		user,
		anonymousToken,
		r.FormValue("client_name"),
		amount,
		r.FormValue("description"),
	)
	if err != nil {
		http.Error(w, "Failed to create invoice", http.StatusInternalServerError)
		return
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

	h.App.Render(w, r, "invoices_list.tmpl", map[string]any{
		"Invoices": invoices,
		"User":     user,
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

	h.App.Render(w, r, "invoice_new.tmpl", map[string]any{
		"Invoice": invoiceView,
		"IsEdit":  true,
		"User":    user,
	})
}

func (h *Handlers) InvoiceUpdatePost(w http.ResponseWriter, r *http.Request) {
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

