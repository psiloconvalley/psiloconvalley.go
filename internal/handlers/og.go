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
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"psiloconvalley/internal/pdf"
	"psiloconvalley/internal/views"
)

// =====================================================================
// OG Image Handlers — Dynamic Open Graph image generation
//
// Each handler:
//   1. Fetches real data (or screenshots a live URL)
//   2. Renders HTML → Chromium → JPEG at ogWidth × ogHeight
//   3. Serves image/jpeg with ogCacheTTL cache headers
//   4. Falls back to static og-default.jpg on any error
//
// These routes are intentionally public — no auth required.
// Crawlers (Twitterbot, facebookexternalhit, etc.) hit these URLs
// when someone shares a link on social media.
// =====================================================================

// ── Shared constants ────────────────────────────────────────────────
const (
	ogWidth    = 1200
	ogHeight   = 630
	ogQuality  = 90
	ogCacheTTL = 300 // seconds — 5 minutes
)

// ── In-memory OG cache ──────────────────────────────────────────────
// Prevents Chromium from firing on every crawler request.
// A single cached JPEG serves all requests within the TTL window.
// Thread-safe via sync.RWMutex — reads don't block each other.
type ogCache struct {
	mu      sync.RWMutex
	data    []byte
	expires time.Time
}

func (c *ogCache) get() ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.data == nil || time.Now().After(c.expires) {
		return nil, false
	}
	return c.data, true
}

func (c *ogCache) set(data []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = data
	c.expires = time.Now().Add(ttl)
}

// One cache per OG endpoint type.
// Invoice/estimate images are keyed by ID so they don't use this cache —
// they rely on HTTP Cache-Control headers instead.
var defaultOGCache = &ogCache{}

// ── View models for OG templates ────────────────────────────────────
// Minimal structs — only what renders on the social card.

type ogLineItem struct {
	Description string
	Amount      string
}

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

// ── Handlers ────────────────────────────────────────────────────────

// OGDefaultImage screenshots the live homepage for the default OG card.
// Cached in memory for 5 minutes — Chromium fires at most once per TTL.
// Route: GET /og/default.jpg
func (h *Handlers) OGDefaultImage(w http.ResponseWriter, r *http.Request) {
	// Serve from cache if fresh
	if cached, ok := defaultOGCache.get(); ok {
		serveJPEG(w, cached)
		return
	}

	// Cache miss — screenshot the live homepage
	url := h.App.BaseURL
	if url == "" {
		url = "https://psiloconvalley.com"
	}

	imgBytes, err := pdf.ScreenshotURL(r.Context(), url, ogWidth, ogHeight)
	if err != nil {
		slog.Error("og default screenshot failed", "url", url, "err", err)
		serveDefaultOGFallback(w, r)
		return
	}

	// Store in cache
	defaultOGCache.set(imgBytes, time.Duration(ogCacheTTL)*time.Second)

	serveJPEG(w, imgBytes)
}

// OGInvoiceImage generates a dynamic OG image for a given invoice ID.
// Route: GET /og/invoice/{id}.jpg
func (h *Handlers) OGInvoiceImage(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimSuffix(chi.URLParam(r, "id"), ".jpg")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		serveDefaultOGFallback(w, r)
		return
	}

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil {
		slog.Warn("og invoice not found", "id", id)
		serveDefaultOGFallback(w, r)
		return
	}

	// Map to OG view model — max 3 line items to fit the card
	invPage := views.MapInvoicePage(inv, items, "og")
	lineItems := mapLineItems(invPage.Items, 3)

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
		serveDefaultOGFallback(w, r)
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
		serveDefaultOGFallback(w, r)
		return
	}

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil {
		slog.Warn("og estimate not found", "id", id)
		serveDefaultOGFallback(w, r)
		return
	}

	invPage := views.MapInvoicePage(inv, items, "og")
	lineItems := mapLineItems(invPage.Items, 3)

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
		serveDefaultOGFallback(w, r)
		return
	}

	serveJPEG(w, imgBytes)
}

// ── Internal helpers ────────────────────────────────────────────────

// mapLineItems converts invoice item views to OG line items.
// Limits to maxItems to fit the social card layout.
func mapLineItems(items []views.InvoiceItemView, maxItems int) []ogLineItem {
	result := make([]ogLineItem, 0, maxItems)
	for i, item := range items {
		if i >= maxItems {
			break
		}
		result = append(result, ogLineItem{
			Description: item.Description,
			Amount:      item.LineTotal,
		})
	}
	return result
}

// renderOGTemplate executes a named template to an HTML string,
// then screenshots it with Chromium at ogWidth × ogHeight.
func (h *Handlers) renderOGTemplate(name string, data any, r *http.Request) ([]byte, error) {
	var buf bytes.Buffer
	if err := h.App.Templates.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, fmt.Errorf("template execute [%s]: %w", name, err)
	}
	return pdf.Screenshot(r.Context(), buf.String(), ogWidth, ogHeight)
}

// serveJPEG writes JPEG bytes with a public cache header.
func serveJPEG(w http.ResponseWriter, img []byte) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control",
		fmt.Sprintf("public, max-age=%d, s-maxage=%d", ogCacheTTL, ogCacheTTL))
	w.Header().Set("Content-Length", strconv.Itoa(len(img)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(img)
}

// serveDefaultOGFallback serves the static fallback OG image.
// Called when DB lookup fails, Chromium errors, or ID is invalid.
// Static file — zero Chromium cost — safe to call from any error path.
func serveDefaultOGFallback(w http.ResponseWriter, r *http.Request) {
	path := "static/img/og-default.jpg"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.ServeFile(w, r, path)
}
