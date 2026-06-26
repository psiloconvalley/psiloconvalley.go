// internal/handlers/og.go
package handlers

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"psiloconvalley/internal/pdf"
	"psiloconvalley/internal/views"
)

// =====================================================================
// OG Image Handlers — Dynamic Open Graph image generation
//
// Each handler:
//   1. Fetches real data from the DB
//   2. Renders an og/ template to an HTML string
//   3. Screenshots it with Chromium at 1200×630
//   4. Serves image/jpeg with a 5-minute cache header
//   5. Falls back to static og-default.jpg on any error
//
// These routes are intentionally public — no auth required.
// The integer ID is the same access control as InvoiceDetail.
// Crawlers (Twitterbot, facebookexternalhit, etc.) hit these URLs
// when someone shares an invoice or estimate link on social media.
// =====================================================================

// ogLineItem is a minimal line item for OG templates.
// We don't need all InvoiceItemView fields — just what renders on the card.
type ogLineItem struct {
	Description string
	Amount      string
}

// ogInvoiceData is the view model for templates/og/invoice.tmpl
type ogInvoiceData struct {
	FromName  string
	FromBiz   string
	ToName    string
	ToSub     string
	DueDate   string
	Total     string
	IsPaid    bool
	LineItems []ogLineItem
}

// ogEstimateData is the view model for templates/og/estimate.tmpl
type ogEstimateData struct {
	FromName   string
	FromBiz    string
	ToName     string
	ToSub      string
	DueDate    string
	Total      string
	IsApproved bool
	LineItems  []ogLineItem
}

// OGInvoiceImage generates a dynamic OG image for a given invoice ID.
// Route: GET /og/invoice/{id}.jpg
func (h *Handlers) OGInvoiceImage(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimSuffix(chi.URLParam(r, "id"), ".jpg")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		serveDefaultOG(w, r)
		return
	}

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil {
		slog.Warn("og invoice not found", "id", id)
		serveDefaultOG(w, r)
		return
	}

	// Map to OG view model — max 3 line items to fit the card
	invPage := views.MapInvoicePage(inv, items, "og")
	lineItems := make([]ogLineItem, 0, 3)
	for i, item := range invPage.Items {
		if i >= 3 {
			break
		}
		lineItems = append(lineItems, ogLineItem{
			Description: item.Description,
			Amount:      item.LineTotal,
		})
	}

	dueDate := invPage.DueDate
	if dueDate == "" {
		dueDate = "On receipt"
	}

	data := ogInvoiceData{
		FromName:  invPage.CompanyName,
		FromBiz:   invPage.CompanyCity,
		ToName:    invPage.ClientName,
		ToSub:     invPage.ClientEmail,
		DueDate:   dueDate,
		Total:     invPage.Total,
		IsPaid:    inv.Status == "paid",
		LineItems: lineItems,
	}

	imgBytes, err := h.renderOGTemplate("og_invoice.tmpl", data, r)
	if err != nil {
		slog.Error("og invoice render failed", "id", id, "err", err)
		serveDefaultOG(w, r)
		return
	}

	serveJPEG(w, imgBytes)
}

// OGEstimateImage generates a dynamic OG image for a given estimate ID.
// Route: GET /og/estimate/{id}.jpg
func (h *Handlers) OGEstimateImage(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimSuffix(chi.URLParam(r, "id"), ".jpg")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		serveDefaultOG(w, r)
		return
	}

	// Estimates use the same Invoice model with DocumentType="estimate"
	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil {
		slog.Warn("og estimate not found", "id", id)
		serveDefaultOG(w, r)
		return
	}

	invPage := views.MapInvoicePage(inv, items, "og")
	lineItems := make([]ogLineItem, 0, 3)
	for i, item := range invPage.Items {
		if i >= 3 {
			break
		}
		lineItems = append(lineItems, ogLineItem{
			Description: item.Description,
			Amount:      item.LineTotal,
		})
	}

	dueDate := invPage.DueDate
	if dueDate == "" {
		dueDate = "Open"
	}

	data := ogEstimateData{
		FromName:   invPage.CompanyName,
		FromBiz:    invPage.CompanyCity,
		ToName:     invPage.ClientName,
		ToSub:      invPage.ClientEmail,
		DueDate:    dueDate,
		Total:      invPage.Total,
		IsApproved: inv.Status == "approved",
		LineItems:  lineItems,
	}

	imgBytes, err := h.renderOGTemplate("og_estimate.tmpl", data, r)
	if err != nil {
		slog.Error("og estimate render failed", "id", id, "err", err)
		serveDefaultOG(w, r)
		return
	}

	serveJPEG(w, imgBytes)
}

// OGDefaultImage renders the branded default OG card.
// Route: GET /og/default.jpg
func (h *Handlers) OGDefaultImage(w http.ResponseWriter, r *http.Request) {
	imgBytes, err := h.renderOGTemplate("og_default.tmpl", nil, r)
	if err != nil {
		slog.Error("og default render failed", "err", err)
		serveDefaultOG(w, r)
		return
	}
	serveJPEG(w, imgBytes)
}

// ── Internal helpers ──────────────────────────────────────────────────

// renderOGTemplate executes a named template to an HTML string,
// then screenshots it with Chromium at 1200×630.
func (h *Handlers) renderOGTemplate(name string, data any, r *http.Request) ([]byte, error) {
	var buf bytes.Buffer
	if err := h.App.Templates.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, fmt.Errorf("template execute [%s]: %w", name, err)
	}
	return pdf.Screenshot(r.Context(), buf.String(), 1200, 630)
}

// serveJPEG writes JPEG bytes with a 5-minute public cache header.
// 5 minutes = crawlers get fresh data quickly without hammering Chromium.
func serveJPEG(w http.ResponseWriter, img []byte) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=300, s-maxage=300")
	w.Header().Set("Content-Length", strconv.Itoa(len(img)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(img)
}

// serveDefaultOG serves the static fallback OG image.
// Called when DB lookup fails, Chromium errors, or ID is invalid.
// Static file — zero Chromium cost — safe to call from any error path.
func serveDefaultOG(w http.ResponseWriter, r *http.Request) {
	path := "static/img/og-default.jpg"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Static file not yet created — serve a plain 200 with no body
		// rather than a 404 that breaks social cards entirely.
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.ServeFile(w, r, path)
}
