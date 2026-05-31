// internal/handlers/estimate_test.go
package handlers

// Note: fakeInvoiceStore, makeHandlers, and withChiID are defined in
// invoice_test.go — all _test.go files in the same package share types.

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/repo"
)

// =====================================================================
// Helpers
// =====================================================================

func makeEstimateStatusRequest(user *repo.User, estimateID, newStatus string) *http.Request {
	form := url.Values{}
	form.Set("status", newStatus)
	r := httptest.NewRequest(
		http.MethodPost,
		"/estimates/"+estimateID+"/status",
		strings.NewReader(form.Encode()),
	)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = withChiID(r, "id", estimateID)
	if user != nil {
		r = auth.SetUser(r, user)
	}
	return r
}

func makeEstimateConvertRequest(user *repo.User, estimateID string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/estimates/"+estimateID+"/convert", nil)
	r = withChiID(r, "id", estimateID)
	if user != nil {
		r = auth.SetUser(r, user)
	}
	return r
}

// =====================================================================
// Tests — EstimateStatusPost
// =====================================================================

// TestEstimateStatusPost_NotLoggedIn verifies unauthenticated requests
// are redirected to /login.
func TestEstimateStatusPost_NotLoggedIn(t *testing.T) {
	h := makeHandlers(&fakeInvoiceStore{})

	w := httptest.NewRecorder()
	r := makeEstimateStatusRequest(nil, "1", "sent")

	h.EstimateStatusPost(w, r)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

// TestEstimateStatusPost_NotAnEstimate verifies that posting a status
// update to a regular invoice returns 400.
// The estimate status flow must never touch invoice documents.
func TestEstimateStatusPost_NotAnEstimate(t *testing.T) {
	userID := int64(5)
	h := makeHandlers(&fakeInvoiceStore{
		invoice: &repo.Invoice{
			ID:           1,
			UserID:       &userID,
			Status:       "draft",
			DocumentType: "invoice", // ← wrong type
		},
	})

	w := httptest.NewRecorder()
	r := makeEstimateStatusRequest(&repo.User{ID: userID}, "1", "sent")

	h.EstimateStatusPost(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invoice document, got %d", w.Code)
	}
}

// TestEstimateStatusPost_InvalidStatus verifies that submitting a
// completely unknown status string returns 400.
func TestEstimateStatusPost_InvalidStatus(t *testing.T) {
	userID := int64(5)
	h := makeHandlers(&fakeInvoiceStore{
		invoice: &repo.Invoice{
			ID:           1,
			UserID:       &userID,
			Status:       "draft",
			DocumentType: "estimate",
		},
	})

	w := httptest.NewRecorder()
	r := makeEstimateStatusRequest(&repo.User{ID: userID}, "1", "bogus")

	h.EstimateStatusPost(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid status, got %d", w.Code)
	}
}

// TestEstimateStatusPost_InvalidTransition verifies that jumping
// from draft directly to accepted (skipping sent) returns 400.
// Valid path is: draft → sent → accepted.
// This test guards the transition table logic.
func TestEstimateStatusPost_InvalidTransition(t *testing.T) {
	userID := int64(5)
	h := makeHandlers(&fakeInvoiceStore{
		invoice: &repo.Invoice{
			ID:           1,
			UserID:       &userID,
			Status:       "draft",  // draft can only go to sent
			DocumentType: "estimate",
		},
	})

	w := httptest.NewRecorder()
	r := makeEstimateStatusRequest(&repo.User{ID: userID}, "1", "accepted") // ← illegal jump

	h.EstimateStatusPost(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for draft→accepted transition, got %d", w.Code)
	}
}

// =====================================================================
// Tests — EstimateConvertPost
// =====================================================================

// TestEstimateConvertPost_NotLoggedIn verifies unauthenticated requests
// are redirected to /login.
func TestEstimateConvertPost_NotLoggedIn(t *testing.T) {
	h := makeHandlers(&fakeInvoiceStore{})

	w := httptest.NewRecorder()
	r := makeEstimateConvertRequest(nil, "1")

	h.EstimateConvertPost(w, r)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

// TestEstimateConvertPost_NotAnEstimate verifies that attempting to
// convert a regular invoice returns 400.
func TestEstimateConvertPost_NotAnEstimate(t *testing.T) {
	userID := int64(5)
	h := makeHandlers(&fakeInvoiceStore{
		invoice: &repo.Invoice{
			ID:           1,
			UserID:       &userID,
			Status:       "accepted",
			DocumentType: "invoice", // ← wrong type
		},
	})

	w := httptest.NewRecorder()
	r := makeEstimateConvertRequest(&repo.User{ID: userID}, "1")

	h.EstimateConvertPost(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invoice document, got %d", w.Code)
	}
}

// TestEstimateConvertPost_NotAccepted verifies the core business rule:
// only accepted estimates can be converted to invoices.
// A sent, draft, or declined estimate must be blocked.
func TestEstimateConvertPost_NotAccepted(t *testing.T) {
	userID := int64(5)

	statuses := []string{"draft", "sent", "rejected", "expired"}

	for _, status := range statuses {
		t.Run("status="+status, func(t *testing.T) {
			h := makeHandlers(&fakeInvoiceStore{
				invoice: &repo.Invoice{
					ID:           1,
					UserID:       &userID,
					Status:       status, // ← none of these should convert
					DocumentType: "estimate",
				},
			})

			w := httptest.NewRecorder()
			r := makeEstimateConvertRequest(&repo.User{ID: userID}, "1")

			h.EstimateConvertPost(w, r)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status %q: expected 400, got %d", status, w.Code)
			}
		})
	}
}
