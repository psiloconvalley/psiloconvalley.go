// internal/handlers/public_profile.go
// Public-facing business profile page.
// No auth required — accessible by anyone with the URL.
// Three endpoints:
//   GET  /biz/{slug}          — render the public profile page
//   POST /biz/{slug}/quote    — submit a quote request
//   POST /biz/{slug}/invoice  — look up an invoice by number + phone
package handlers

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"
	"psiloconvalley/internal/app"
	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/repo"
)

// PublicProfileGet renders the public business profile page.
func (h *Handlers) PublicProfileGet(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	profile, err := h.App.BizRepo.GetBySlug(r.Context(), slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	displayPhone := profile.Phone
	if displayPhone != "" {
		displayPhone = catalog.FormatPhone(displayPhone)
	}

	location := buildLocation(profile.City, profile.State)

	ogImage := "https://psiloconvalley.com/og/default.jpg"
	if profile.LogoURL != "" {
		ogImage = profile.LogoURL
	}

	h.App.Render(w, r, "public_profile.tmpl", map[string]any{
		"Profile":      profile,
		"DisplayPhone": displayPhone,
		"Location":     location,
		"BaseURL":      h.App.BaseURL,
		"Slug":         slug,
		"Meta": app.PageMeta{
			Title:       profile.Name + " | PSILOCONVALLEY",
			Description: profile.Name + " — " + location + ". Request a quote or pay an invoice.",
			TwitterDesc: profile.Name + " — Request a quote or pay an invoice.",
			Canonical:   h.App.BaseURL + "/biz/" + slug,
			OGImage:     ogImage,
			Robots:      "index,follow",
			IsPublic:    true,
		},
	})
}

// PublicQuotePost handles the "Request a Quote" form submission.
func (h *Handlers) PublicQuotePost(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	profile, err := h.App.BizRepo.GetBySlug(r.Context(), slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	clientName := strings.TrimSpace(r.FormValue("client_name"))
	clientPhone := strings.TrimSpace(r.FormValue("client_phone"))
	description := strings.TrimSpace(r.FormValue("description"))

	if clientName == "" {
		h.renderPublicProfile(w, r, profile, slug, "Name is required.", "", false)
		return
	}

	qr := &repo.QuoteRequest{
		BusinessProfileID: profile.ID,
		ClientName:        clientName,
		ClientPhone:       catalog.FormatPhone(clientPhone),
		Description:       description,
	}

	_, err = h.App.QuoteRequestRepo.Create(r.Context(), qr)
	if err != nil {
		slog.Error("quote request create failed", "err", err, "slug", slug)
		h.renderPublicProfile(w, r, profile, slug, "Something went wrong. Please try again.", "", false)
		return
	}

	// Send email notification to business owner
	if profile.Email != "" {
		go func() {
			subject := "New Quote Request from " + clientName
			body := "You have a new quote request.\n\n" +
				"Name: " + clientName + "\n" +
				"Phone: " + clientPhone + "\n" +
				"Description: " + description + "\n\n" +
				"View in your dashboard: " + h.App.BaseURL + "/dashboard"
				if err := h.App.Mailer.SendNotification(profile.Email, subject, body); err != nil {
				slog.Warn("quote request email failed", "err", err, "to", profile.Email)
			}
		}()
	}

	h.renderPublicProfile(w, r, profile, slug, "", "Quote request sent! They will be in touch.", false)
}

// PublicInvoiceLookupPost handles the "Pay an Invoice" form submission.
func (h *Handlers) PublicInvoiceLookupPost(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	profile, err := h.App.BizRepo.GetBySlug(r.Context(), slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	invoiceNumber := strings.TrimSpace(r.FormValue("invoice_number"))
	clientPhone := strings.TrimSpace(r.FormValue("client_phone"))

	if invoiceNumber == "" {
		h.renderPublicProfile(w, r, profile, slug, "", "", false)
		return
	}

	inv, err := h.findInvoiceForPublicProfile(r, profile.ID, invoiceNumber, clientPhone)
	if err != nil || inv == nil {
		h.renderPublicProfile(w, r, profile, slug, "", "Invoice not found. Please check your invoice number and phone.", true)
		return
	}

	token, err := h.App.InvRepo.EnsurePublicToken(r.Context(), inv.ID)
	if err != nil {
		slog.Error("public token failed for invoice lookup", "err", err, "invoice_id", inv.ID)
		h.renderPublicProfile(w, r, profile, slug, "", "Something went wrong. Please try again.", true)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/invoices/%d?access=%s", inv.ID, token), http.StatusSeeOther)
}

// renderPublicProfile is the shared render helper.
func (h *Handlers) renderPublicProfile(
	w http.ResponseWriter, r *http.Request,
	profile *repo.BusinessProfile, slug string,
	quoteError string, quoteSuccess string,
	invoiceError bool,
) {
	displayPhone := profile.Phone
	if displayPhone != "" {
		displayPhone = catalog.FormatPhone(displayPhone)
	}

	h.App.Render(w, r, "public_profile.tmpl", map[string]any{
		"Profile":      profile,
		"DisplayPhone": displayPhone,
		"Location":     buildLocation(profile.City, profile.State),
		"BaseURL":      h.App.BaseURL,
		"Slug":         slug,
		"QuoteError":   quoteError,
		"QuoteSuccess": quoteSuccess,
		"InvoiceError": invoiceError,
	})
}

// findInvoiceForPublicProfile looks up an invoice by number within a business profile.
// Verifies client phone matches for security.
func (h *Handlers) findInvoiceForPublicProfile(
	r *http.Request,
	bizProfileID int64,
	invoiceNumber string,
	clientPhone string,
) (*repo.Invoice, error) {
	var inv repo.Invoice
	var uID sql.NullInt64
	err := h.App.DB().QueryRowContext(r.Context(), `
		SELECT id, user_id, client_name, invoice_number, status,
		       COALESCE(public_token, '') AS public_token
		FROM invoices
		WHERE invoice_number = $1
		AND business_profile_id = $2
		AND document_type = 'invoice'
		LIMIT 1
	`, invoiceNumber, bizProfileID).Scan(
		&inv.ID, &uID, &inv.ClientName, &inv.InvoiceNumber,
		&inv.Status, &inv.PublicToken,
	)
	if err != nil {
		return nil, err
	}
	if uID.Valid {
		inv.UserID = &uID.Int64
	}

	// Cross-reference phone if provided
	if clientPhone != "" {
		formattedPhone := catalog.FormatPhone(clientPhone)
		var phoneOnFile sql.NullString
		_ = h.App.DB().QueryRowContext(r.Context(), `
			SELECT c.phone
			FROM clients c
			INNER JOIN invoices i ON i.client_id = c.id
			WHERE i.id = $1
		`, inv.ID).Scan(&phoneOnFile)

		if phoneOnFile.Valid && phoneOnFile.String != "" {
			if phoneOnFile.String != formattedPhone {
				return nil, sql.ErrNoRows
			}
		}
		// No phone on file → allow access — don't block payment
	}

	return &inv, nil
}

// buildLocation creates "City, State" display string.
func buildLocation(city, state string) string {
	parts := []string{}
	if city != "" {
		parts = append(parts, city)
	}
	if state != "" {
		parts = append(parts, state)
	}
	return strings.Join(parts, ", ")
}

// Suppress unused import warnings
// PublicProfileQR serves a QR code PNG for the business profile URL.
// GET /biz/{slug}/qr.png
func (h *Handlers) PublicProfileQR(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	// Verify the slug exists
	_, err := h.App.BizRepo.GetBySlug(r.Context(), slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	profileURL := h.App.BaseURL + "/biz/" + slug

	png, err := qrcode.Encode(profileURL, qrcode.Medium, 512)
	if err != nil {
		slog.Error("qr code generation failed", "err", err, "slug", slug)
		http.Error(w, "QR generation failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(png)
}
