// internal/handlers/endorsement.go
// Endorsement flow — token-based client testimonials for business profiles.
//
// Public routes (no auth):
//   GET  /endorse/{token}   — client endorsement form
//   POST /endorse/{token}   — client submits endorsement
//   POST /endorse/{token}/decline — client declines
//
// Protected routes (owner only):
//   POST /invoices/{id}/endorse  — owner requests endorsement
//   GET  /endorsements           — owner management page
//   POST /endorsements/{id}/delete — owner deletes endorsement
package handlers

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"psiloconvalley/internal/app"
	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/mailer"
)

// ── Owner: Request Endorsement ────────────────────────────────────────────
// POST /invoices/{id}/endorse
// Owner taps "Request Endorsement" on a paid invoice.
// Looks up the client email from the invoice, sends the request email.

func (h *Handlers) EndorsementRequestPost(w http.ResponseWriter, r *http.Request) {
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

	// Load invoice — verify ownership
	inv, _, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil {
		http.NotFound(w, r)
		return
	}
	if inv.UserID == nil || *inv.UserID != user.ID {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}
	if inv.Status != "paid" {
		http.Error(w, "Endorsements can only be requested for paid invoices", http.StatusBadRequest)
		return
	}
	if inv.ClientEmail == "" {
		// No email on file — redirect back with error
		http.Redirect(w, r, fmt.Sprintf("/invoices/%d?endorse=no_email", id), http.StatusSeeOther)
		return
	}

	// Load business profile for the owner
	biz, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID)
	if err != nil || biz == nil {
		slog.Error("endorsement request: no business profile", "user_id", user.ID)
		http.Error(w, "Business profile not found", http.StatusInternalServerError)
		return
	}

	// Check if an endorsement already exists for this invoice
	existing, _ := h.App.EndorsementRepo.GetByInvoiceID(r.Context(), id)
	if existing != nil {
		if existing.Status == "submitted" {
			http.Redirect(w, r, fmt.Sprintf("/invoices/%d?endorse=already_received", id), http.StatusSeeOther)
			return
		}
		if existing.Status == "pending" {
			// Resend the existing endorsement request email
			endorseURL := fmt.Sprintf("%s/endorse/%s", h.App.BaseURL, existing.Token)
			go func() {
				if err := h.App.Mailer.SendEndorsementRequest(inv.ClientEmail, mailer.EndorsementRequestData{
					ClientName:   inv.ClientName,
					BusinessName: biz.Name,
					EndorseURL:   endorseURL,
				}); err != nil {
					slog.Warn("endorsement resend failed", "to", inv.ClientEmail, "err", err)
				}
			}()
			http.Redirect(w, r, fmt.Sprintf("/invoices/%d?endorse=resent", id), http.StatusSeeOther)
			return
		}
		// declined — allow creating a new one (fall through)
	}

	// Create the endorsement record and get the token
	token, err := h.App.EndorsementRepo.RequestEndorsement(r.Context(), biz.ID, &id)
	if err != nil {
		slog.Error("endorsement request: create failed", "user_id", user.ID, "invoice_id", id, "err", err)
		http.Error(w, "Failed to create endorsement request", http.StatusInternalServerError)
		return
	}

	// Send the email to the client
	endorseURL := fmt.Sprintf("%s/endorse/%s", h.App.BaseURL, token)
	go func() {
		if err := h.App.Mailer.SendEndorsementRequest(inv.ClientEmail, mailer.EndorsementRequestData{
			ClientName:   inv.ClientName,
			BusinessName: biz.Name,
			EndorseURL:   endorseURL,
		}); err != nil {
			slog.Warn("endorsement request: email failed", "to", inv.ClientEmail, "invoice_id", id, "err", err)
		}
	}()

	http.Redirect(w, r, fmt.Sprintf("/invoices/%d?endorse=sent", id), http.StatusSeeOther)
}

// ── Public: Endorsement Form ──────────────────────────────────────────────
// GET /endorse/{token}
// Client opens the endorsement link from email.

func (h *Handlers) EndorsementFormGet(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	endorsement, err := h.App.EndorsementRepo.GetByToken(r.Context(), token)
	if err == sql.ErrNoRows || endorsement == nil {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("endorsement form: get by token failed", "err", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	// Already submitted or declined — show thank you state
	if endorsement.Status == "submitted" {
		h.App.Render(w, r, "endorse.tmpl", map[string]any{
			"Submitted": true,
			"Endorsement": endorsement,
			"Meta": app.AuthMeta("Thank You | PSILOCONVALLEY"),
		})
		return
	}
	if endorsement.Status == "declined" {
		h.App.Render(w, r, "endorse.tmpl", map[string]any{
			"Declined": true,
			"Meta":     app.AuthMeta("Endorsement | PSILOCONVALLEY"),
		})
		return
	}

	// Load business profile for display
	biz, err := h.App.BizRepo.GetByID(r.Context(), endorsement.BusinessProfileID)
	if err != nil {
		slog.Error("endorsement form: load biz profile failed", "err", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	h.App.Render(w, r, "endorse.tmpl", map[string]any{
		"Token":       token,
		"Business":    biz,
		"Endorsement": endorsement,
		"Meta":        app.AuthMeta("Leave an Endorsement | PSILOCONVALLEY"),
	})
}

// ── Public: Submit Endorsement ────────────────────────────────────────────
// POST /endorse/{token}
// Client submits their endorsement. Goes live immediately.

func (h *Handlers) EndorsementSubmitPost(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	endorsement, err := h.App.EndorsementRepo.GetByToken(r.Context(), token)
	if err == sql.ErrNoRows || endorsement == nil {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	if endorsement.Status != "pending" {
		http.Redirect(w, r, "/endorse/"+token, http.StatusSeeOther)
		return
	}

	// Parse and validate form
	ratingStr := r.FormValue("rating")
	rating, err := strconv.Atoi(ratingStr)
	if err != nil || rating < 1 || rating > 5 {
		biz, _ := h.App.BizRepo.GetByID(r.Context(), endorsement.BusinessProfileID)
		h.App.Render(w, r, "endorse.tmpl", map[string]any{
			"Token":       token,
			"Business":    biz,
			"Endorsement": endorsement,
			"Error":       "Please select a star rating.",
			"Meta":        app.AuthMeta("Leave an Endorsement | PSILOCONVALLEY"),
		})
		return
	}

	name := strings.TrimSpace(r.FormValue("endorser_name"))
	if name == "" {
		biz, _ := h.App.BizRepo.GetByID(r.Context(), endorsement.BusinessProfileID)
		h.App.Render(w, r, "endorse.tmpl", map[string]any{
			"Token":       token,
			"Business":    biz,
			"Endorsement": endorsement,
			"Error":       "Please enter your name.",
			"Meta":        app.AuthMeta("Leave an Endorsement | PSILOCONVALLEY"),
		})
		return
	}

	location := strings.TrimSpace(r.FormValue("endorser_location"))
	body := strings.TrimSpace(r.FormValue("body"))

	if err := h.App.EndorsementRepo.Submit(r.Context(), token, name, location, body, rating); err != nil {
		slog.Error("endorsement submit failed", "err", err)
		http.Error(w, "Failed to submit endorsement", http.StatusInternalServerError)
		return
	}

	// Reload to show submitted state
	endorsement.EndorserName = name
	endorsement.EndorserLocation = location
	endorsement.Rating = rating
	endorsement.Body = body
	endorsement.Status = "submitted"

	h.App.Render(w, r, "endorse.tmpl", map[string]any{
		"Submitted":   true,
		"Endorsement": endorsement,
		"Meta":        app.AuthMeta("Thank You | PSILOCONVALLEY"),
	})
}

// ── Public: Decline Endorsement ───────────────────────────────────────────
// POST /endorse/{token}/decline
// Client chose "not at this time" — marks as declined, never shown publicly.

func (h *Handlers) EndorsementDeclinePost(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	_ = h.App.EndorsementRepo.Decline(r.Context(), token)

	h.App.Render(w, r, "endorse.tmpl", map[string]any{
		"Declined": true,
		"Meta":     app.AuthMeta("Endorsement | PSILOCONVALLEY"),
	})
}

// ── Owner: Management Page ────────────────────────────────────────────────
// GET /endorsements
// Salvador sees all his endorsements — pending, submitted, declined.
// Can delete any endorsement.

func (h *Handlers) EndorsementsListGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	endorsements, err := h.App.EndorsementRepo.ListByUserID(r.Context(), user.ID)
	if err != nil {
		slog.Error("endorsements list failed", "user_id", user.ID, "err", err)
		endorsements = nil
	}

	// Count by status for the header
	var pendingCount, submittedCount int
	for _, e := range endorsements {
		switch e.Status {
		case "pending":
			pendingCount++
		case "submitted":
			submittedCount++
		}
	}

	h.App.Render(w, r, "endorsements.tmpl", map[string]any{
		"Endorsements":   endorsements,
		"PendingCount":   pendingCount,
		"SubmittedCount": submittedCount,
		"BaseURL":        h.App.BaseURL,
		"Meta":           app.AuthMeta("Endorsements | PSILOCONVALLEY"),
	})
}

// ── Owner: Delete Endorsement ─────────────────────────────────────────────
// POST /endorsements/{id}/delete
// Owner removes an endorsement. Ownership verified in repo.

func (h *Handlers) EndorsementDeletePost(w http.ResponseWriter, r *http.Request) {
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

	if err := h.App.EndorsementRepo.Delete(r.Context(), id, user.ID); err != nil {
		slog.Warn("endorsement delete failed",
			"user_id", user.ID, "endorsement_id", id, "err", err)
	}

	http.Redirect(w, r, "/endorsements", http.StatusSeeOther)
}
