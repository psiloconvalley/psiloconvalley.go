// internal/handlers/invoice_test.go
package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"


	"psiloconvalley/internal/app"
	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/repo"
	"github.com/go-chi/chi/v5"

)

// =====================================================================
// fakeInvoiceStore — implements repo.InvoiceStore for testing.
//
// Only the methods relevant to each test need real behaviour.
// Everything else returns zero values and nil errors by default.
// This is called a "stub" — it satisfies the interface contract
// without touching a database.
// =====================================================================

type fakeInvoiceStore struct {
	// Seed these per-test to control what the fake returns
	invoice    *repo.Invoice
	itemsErr   error
	deleteErr  error
	deleteCalled bool
}

func (f *fakeInvoiceStore) GetInvoiceWithItems(
	ctx context.Context,
	id int64,
) (*repo.Invoice, []repo.InvoiceItem, error) {
	return f.invoice, nil, f.itemsErr
}

func (f *fakeInvoiceStore) DeleteDraftInvoice(
	ctx context.Context,
	id int64,
	userID int64,
) error {
	f.deleteCalled = true
	return f.deleteErr
}

// ---- Remaining interface methods — stubs, not used in these tests ----

func (f *fakeInvoiceStore) CreateWithToken(ctx context.Context, user *repo.User, anonymousToken, clientName string, amount float64, description string) (int64, error) {
	return 0, nil
}
func (f *fakeInvoiceStore) CreateInvoice(ctx context.Context, inv *repo.Invoice, items []repo.InvoiceItem, anonymousToken string) (int64, error) {
	return 0, nil
}
func (f *fakeInvoiceStore) ListInvoices(ctx context.Context, limit, offset int, userID *int64) ([]repo.Invoice, error) {
	return nil, nil
}
func (f *fakeInvoiceStore) ListEstimates(ctx context.Context, limit, offset int, userID int64) ([]repo.Invoice, error) {
	return nil, nil
}
func (f *fakeInvoiceStore) InvoiceNumberExists(ctx context.Context, number string, userID int64) (bool, error) {
	return false, nil
}
func (f *fakeInvoiceStore) UpdateInvoice(ctx context.Context, inv *repo.Invoice, items []repo.InvoiceItem) error {
	return nil
}
func (f *fakeInvoiceStore) UpdateInvoiceStatus(ctx context.Context, id int64, newStatus string, userID int64) error {
	return nil
}
func (f *fakeInvoiceStore) EnsurePublicToken(ctx context.Context, invoiceID int64) (string, error) {
	return "", nil
}
func (f *fakeInvoiceStore) GetDashboardStats(ctx context.Context, userID int64) (*repo.DashboardStats, error) {
	return &repo.DashboardStats{}, nil
}
func (f *fakeInvoiceStore) GetAdminStats(ctx context.Context, db *sql.DB) (*repo.AdminStats, error) {
	return &repo.AdminStats{}, nil
}
func (f *fakeInvoiceStore) ListInvoicesForReport(ctx context.Context, userID int64, start, end time.Time, status string) ([]repo.InvoiceReportRow, error) {
	return nil, nil
}
func (f *fakeInvoiceStore) GetClientScorecards(ctx context.Context, userID int64) ([]repo.ClientScorecard, error) {
	return nil, nil
}

// Helpers
// makeDeleteRequest builds a fake POST to /invoices/{id}/delete
// with a logged-in user injected into the context.
func makeDeleteRequest(user *repo.User, invoiceID string) *http.Request {
	form := url.Values{}
	form.Set("confirm_number", "INV-0001")
	r := httptest.NewRequest(
		http.MethodPost,
		"/invoices/"+invoiceID+"/delete",
		strings.NewReader(form.Encode()),
	)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Inject chi URL param so chi.URLParam(r, "id") works
	r = withChiID(r, "id", invoiceID)

	if user != nil {
		r = auth.SetUser(r, user)
	}
	return r
}

// withChiID injects a chi route param into the request context.
// This mimics what chi's router does when it matches a route like
// /invoices/{id}/delete — without needing a real router in tests.
func withChiID(r *http.Request, key, val string) *http.Request {
	// chi stores URL params in the request context using its own key.
	// The clean way to set them in tests is via chi.NewRouteContext().
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, chiCtx))
}

// makeHandlers builds a minimal Handlers with a fake InvoiceStore.
// No real database, no real app — just enough to run the handler.
func makeHandlers(store *fakeInvoiceStore) *Handlers {
	a := &app.App{}
	a.InvRepo = store
	return &Handlers{App: a}
}

// =====================================================================
// Tests — InvoiceDeletePost
// =====================================================================

// TestInvoiceDeletePost_NotLoggedIn verifies that an unauthenticated
// request is redirected to /login.
func TestInvoiceDeletePost_NotLoggedIn(t *testing.T) {
	store := &fakeInvoiceStore{}
	h := makeHandlers(store)

	w := httptest.NewRecorder()
	r := makeDeleteRequest(nil, "1")

	h.InvoiceDeletePost(w, r)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

// TestInvoiceDeletePost_NonDraftBlocked verifies that attempting to
// delete a sent invoice returns 400 Bad Request.
// The draft-only rule must be enforced at the handler level.
func TestInvoiceDeletePost_NonDraftBlocked(t *testing.T) {
	userID := int64(5)
	store := &fakeInvoiceStore{
		invoice: &repo.Invoice{
			ID:     1,
			UserID: &userID,
			Status: "sent", // ← not a draft
		},
	}
	h := makeHandlers(store)

	user := &repo.User{ID: userID}
	w := httptest.NewRecorder()
	r := makeDeleteRequest(user, "1")

	h.InvoiceDeletePost(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-draft delete, got %d", w.Code)
	}
	if store.deleteCalled {
		t.Error("DeleteDraftInvoice should not have been called for a non-draft invoice")
	}
}

// TestInvoiceDeletePost_DraftSucceeds verifies that deleting a draft
// invoice calls through to the repo and returns a redirect.
func TestInvoiceDeletePost_DraftSucceeds(t *testing.T) {
	userID := int64(5)
	store := &fakeInvoiceStore{
		invoice: &repo.Invoice{
			ID:            1,
			UserID:        &userID,
			Status:        "draft",
			InvoiceNumber: "INV-0001",
			DocumentType:  "invoice",
		},
	}
	h := makeHandlers(store)

	user := &repo.User{ID: userID}
	w := httptest.NewRecorder()
	r := makeDeleteRequest(user, "1")

	h.InvoiceDeletePost(w, r)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect after delete, got %d", w.Code)
	}
	if !store.deleteCalled {
		t.Error("expected DeleteDraftInvoice to be called for a draft invoice")
	}
}
