// internal/handlers/admin_test.go
package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/repo"
)

// makeAdminRequest is a helper that builds a fake GET request
// and optionally injects a user into the request context.
// Pass nil for user to simulate an unauthenticated visitor.
func makeAdminRequest(user *repo.User) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/admin/analytics", nil)
	if user != nil {
		r = auth.SetUser(r, user)
	}
	return r
}

// TestAdminAnalytics_NilUser verifies that an unauthenticated visitor
// gets a 404 — not a 403, not a redirect, not a data leak.
func TestAdminAnalytics_NilUser(t *testing.T) {
	h := &Handlers{} // App is nil — safe, handler returns before using it

	w := httptest.NewRecorder()
	r := makeAdminRequest(nil) // no user in context

	h.AdminAnalytics(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nil user, got %d", w.Code)
	}
}

// TestAdminAnalytics_NonAdminUser verifies that a valid logged-in user
// who is NOT user ID 1 also gets a 404.
// This is the "curious user guesses the URL" scenario.
func TestAdminAnalytics_NonAdminUser(t *testing.T) {
	h := &Handlers{} // App is nil — safe, handler returns before using it

	w := httptest.NewRecorder()
	r := makeAdminRequest(&repo.User{ID: 99, Email: "notadmin@example.com"})

	h.AdminAnalytics(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-admin user, got %d", w.Code)
	}
}

// TestAdminAnalytics_UserIDZero verifies that a user with ID 0
// (malformed session) is also blocked.
func TestAdminAnalytics_UserIDZero(t *testing.T) {
	h := &Handlers{}

	w := httptest.NewRecorder()
	r := makeAdminRequest(&repo.User{ID: 0})

	h.AdminAnalytics(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for user ID 0, got %d", w.Code)
	}
}
