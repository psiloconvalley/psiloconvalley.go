// internal/handlers/invoice.go
package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"psiloconvalley/internal/auth"
)

// =====================================================================
// 1. PUBLIC / FREEMIUM ROUTES
// =====================================================================

// InvoiceNewGet renders the creation form. Accessible to everyone.
func (h *Handlers) InvoiceNewGet(w http.ResponseWriter, r *http.Request) {
	// If anonymous and reached max free invoices, redirect to register
	userID, isLoggedIn := auth.GetSessionUserID(r)
	if !isLoggedIn && auth.AnonLimitReached(r) {
		http.Redirect(w, r, "/register?reason=limit", http.StatusSeeOther)
		return
	}

	data := map[string]interface{}{
		"IsLoggedIn": isLoggedIn,
		"UserID":     userID,
	}
	h.App.Views.Render(w, r, "invoice_new.tmpl", data)
}

// InvoiceCreatePost handles form submission for both users and guests.
func (h *Handlers) InvoiceCreatePost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	userID, isLoggedIn := auth.GetSessionUserID(r)

	// Enforce freemium limit for anonymous users
	if !isLoggedIn {
		if auth.AnonLimitReached(r) {
			http.Redirect(w, r, "/register?reason=limit", http.StatusSeeOther)
			return
		}
		count := auth.GetAnonInvoiceCount(r)
		auth.SetAnonInvoiceCount(w, count+1)
	}

	amount, _ := strconv.ParseFloat(r.FormValue("amount"), 64)

	// Create invoice record
	invoiceID, err := h.App.InvoiceRepo.Create(r.Context(), userID, r.FormValue("client_name"), amount, r.FormValue("description"))
	if err != nil {
		http.Error(w, "Failed to create invoice", http.StatusInternalServerError)
		return
	}

	// Redirect to the view page
	http.Redirect(w, r, "/invoices/"+strconv.FormatInt(invoiceID, 10), http.StatusSeeOther)
}

// InvoiceDetail displays a specific invoice. Guarded by Zero-Trust checks.
func (h *Handlers) InvoiceDetail(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	invoiceID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		http.Error(w, "Invalid invoice ID", http.StatusBadRequest)
		return
	}

	invoice, err := h.App.InvoiceRepo.GetByID(r.Context(), invoiceID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Invoice not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// =========================================================================
	// MIT-LEVEL SECURITY GATE: ZERO-TRUST OWNERSHIP CHECK
	// =========================================================================
	userID, isLoggedIn := auth.GetSessionUserID(r)

	// CASE 1: Invoice belongs to a registered user (e.g., YOU).
	// If visitor is not logged in, OR logged in as someone else -> BLOCK.
	if invoice.UserID > 0 {
		if !isLoggedIn || invoice.UserID != userID {
			http.Redirect(w, r, "/login?return_to=/invoices/"+idParam, http.StatusSeeOther)
			return
		}
	}

	// CASE 2: Invoice was created anonymously.
	// If a logged-in user tries to browse anonymous invoices -> BLOCK.
	if invoice.UserID == 0 && isLoggedIn {
		http.Error(w, "Forbidden: Invalid Access Context", http.StatusForbidden)
		return
	}
	// =========================================================================

	h.App.Views.Render(w, r, "invoice_detail.tmpl", map[string]interface{}{
		"Invoice":    invoice,
		"IsLoggedIn": isLoggedIn,
	})
}

// InvoicePDFGet generates a PDF. Uses identical security gate to Detail.
func (h *Handlers) InvoicePDFGet(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	invoiceID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		http.Error(w, "Invalid invoice ID", http.StatusBadRequest)
		return
	}

	invoice, err := h.App.InvoiceRepo.GetByID(r.Context(), invoiceID)
	if err != nil || invoice == nil {
		http.Error(w, "Invoice not found", http.StatusNotFound)
		return
	}

	userID, isLoggedIn := auth.GetSessionUserID(r)
	if invoice.UserID > 0 && (!isLoggedIn || invoice.UserID != userID) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if invoice.UserID == 0 && isLoggedIn {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Generate and serve PDF
	pdfBytes, err := h.App.PDFService.GenerateInvoicePDF(invoice)
	if err != nil {
		http.Error(w, "Failed to generate PDF", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=invoice_"+idParam+".pdf")
	w.Write(pdfBytes)
}

// =====================================================================
// 2. PROTECTED MANAGEMENT ROUTES (Requires Login Middleware)
// =====================================================================

// InvoicesList shows all invoices owned by the logged-in user.
func (h *Handlers) InvoicesList(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetSessionUserID(r)

	// Only fetch invoices matching this user's ID
	invoices, err := h.App.InvoiceRepo.ListByUserID(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to load invoices", http.StatusInternalServerError)
		return
	}

	h.App.Views.Render(w, r, "invoices_list.tmpl", map[string]interface{}{
		"Invoices": invoices,
	})
}

// InvoiceEditGet renders the edit form. Guards against editing others' invoices.
func (h *Handlers) InvoiceEditGet(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetSessionUserID(r)
	invoiceID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	invoice, err := h.App.InvoiceRepo.GetByID(r.Context(), invoiceID)
	if err != nil || invoice.UserID != userID {
		http.Error(w, "Unauthorized or Not Found", http.StatusForbidden)
		return
	}

	h.App.Views.Render(w, r, "invoice_new.tmpl", map[string]interface{}{
		"Invoice": invoice,
		"IsEdit":  true,
	})
}

// InvoiceUpdatePost processes edits. Strict ownership check before DB update.
func (h *Handlers) InvoiceUpdatePost(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetSessionUserID(r)
	invoiceID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	// Verify ownership before modifying
	invoice, err := h.App.InvoiceRepo.GetByID(r.Context(), invoiceID)
	if err != nil || invoice.UserID != userID {
		http.Error(w, "Unauthorized attempt to modify record", http.StatusForbidden)
		return
	}

	amount, _ := strconv.ParseFloat(r.FormValue("amount"), 64)
	err = h.App.InvoiceRepo.Update(r.Context(), invoiceID, r.FormValue("client_name"), amount, r.FormValue("description"))
	if err != nil {
		http.Error(w, "Update failed", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/invoices/"+strconv.FormatInt(invoiceID, 10), http.StatusSeeOther)
}

// InvoiceStatusPost updates payment status (paid, pending).
func (h *Handlers) InvoiceStatusPost(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetSessionUserID(r)
	invoiceID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	invoice, err := h.App.InvoiceRepo.GetByID(r.Context(), invoiceID)
	if err != nil || invoice.UserID != userID {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	newStatus := r.FormValue("status")
	if err := h.App.InvoiceRepo.UpdateStatus(r.Context(), invoiceID, newStatus); err != nil {
		http.Error(w, "Failed to update status", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/invoices/"+strconv.FormatInt(invoiceID, 10), http.StatusSeeOther)
}

// InvoiceSendGet renders the email sending prompt.
func (h *Handlers) InvoiceSendGet(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetSessionUserID(r)
	invoiceID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	invoice, err := h.App.InvoiceRepo.GetByID(r.Context(), invoiceID)
	if err != nil || invoice.UserID != userID {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	h.App.Views.Render(w, r, "invoice_send.tmpl", map[string]interface{}{
		"Invoice": invoice,
	})
}

// InvoiceSendPost dispatches invoice via email.
func (h *Handlers) InvoiceSendPost(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetSessionUserID(r)
	invoiceID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	invoice, err := h.App.InvoiceRepo.GetByID(r.Context(), invoiceID)
	if err != nil || invoice.UserID != userID {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	recipient := r.FormValue("email")
	err = h.App.Mailer.SendInvoice(recipient, invoice)
	if err != nil {
		http.Error(w, "Failed to send email", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/invoices/"+strconv.FormatInt(invoiceID, 10)+"?sent=true", http.StatusSeeOther)
}

// InvoiceDuplicateGet clones an existing invoice for fast recreation.
func (h *Handlers) InvoiceDuplicateGet(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetSessionUserID(r)
	invoiceID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	invoice, err := h.App.InvoiceRepo.GetByID(r.Context(), invoiceID)
	if err != nil || invoice.UserID != userID {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	newID, err := h.App.InvoiceRepo.Duplicate(r.Context(), invoiceID, userID)
	if err != nil {
		http.Error(w, "Failed to duplicate", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/invoices/"+strconv.FormatInt(newID, 10)+"/edit", http.StatusSeeOther)
}
