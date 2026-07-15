// internal/handlers/invoice_pdf.go
package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/pdf"
	"psiloconvalley/internal/service"
	"psiloconvalley/internal/views"
)

// InvoicePDFGet generates and streams a PDF for the given invoice.
// Logo is fetched and inlined as base64 — headless Chrome cannot
// reliably fetch external images during PDF generation.
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

	// Inline logo as base64 so headless Chrome needs no outbound network.
	logoStr := string(invoiceView.LogoURL)
	if logoStr != "" {
		if strings.HasPrefix(logoStr, "/static/") {
			// Local disk (LocalStore)
			filePath := "." + logoStr
			if b, err := os.ReadFile(filePath); err == nil {
				mime := http.DetectContentType(b)
				invoiceView.LogoURL = template.URL("data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b))
			} else {
				slog.Warn("pdf logo read from disk failed", "invoice_id", id, "err", err)
			}
		} else if strings.HasPrefix(logoStr, "http") {
			// Remote (SupabaseStore)
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(logoStr)
			if err != nil {
				slog.Warn("pdf logo fetch failed", "invoice_id", id, "err", err)
			} else {
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					slog.Warn("pdf logo fetch non-200", "invoice_id", id, "status", resp.StatusCode, "url", logoStr)
				} else {
					b, err := io.ReadAll(resp.Body)
					if err != nil {
						slog.Warn("pdf logo read failed", "invoice_id", id, "err", err)
					} else {
						mime := http.DetectContentType(b)
						invoiceView.LogoURL = template.URL("data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b))
						slog.Info("pdf logo inlined", "invoice_id", id, "bytes", len(b))
					}
				}
			}
		}
	}

	// Branding footer: hidden for Growth and Pro plans.
	showBranding := true
	if inv.UserID != nil {
		if owner, err := h.App.UserRepo.GetByID(*inv.UserID); err == nil && owner != nil {
			if catalog.IsPaid(owner.Plan) {
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

	if err := h.App.Templates.ExecuteTemplate(
		&buf,
		service.InvoiceTemplateName(invoiceView.TemplateID),
		templateData,
	); err != nil {
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
	w.Header().Set("Content-Disposition",
		"inline; filename=invoice-"+strconv.FormatInt(id, 10)+".pdf")
	_, _ = w.Write(pdfBytes)
}
