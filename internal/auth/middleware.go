package auth

import (
	"context"
	"net/http"

	"psiloconvalley/internal/repo"
)

// =====================================================================
// Context Keys
// =====================================================================

// contextKey is a private type to prevent collisions in context values.
// This is the Go standard pattern for context keys.
type contextKey string

const userContextKey contextKey = "user"

// =====================================================================
// Context Helpers
// =====================================================================

// SetUser stores a User in the request context.
//
// Called by middleware after validating the session.
// Downstream handlers retrieve it with GetUser().
func SetUser(r *http.Request, user *repo.User) *http.Request {
	ctx := context.WithValue(r.Context(), userContextKey, user)
	return r.WithContext(ctx)
}

// GetUser retrieves the authenticated User from the request context.
//
// Returns nil if no user is logged in (anonymous visitor).
// Handlers should always nil-check the return value.
func GetUser(r *http.Request) *repo.User {
	user, ok := r.Context().Value(userContextKey).(*repo.User)
	if !ok {
		return nil
	}
	return user
}

// =====================================================================
// Middleware
// =====================================================================

// LoadUser is middleware that checks for a valid session cookie,
// loads the user from the database, and stores it in the request
// context for downstream handlers.
//
// This runs on EVERY request. It does NOT block unauthenticated
// requests — it simply makes the user available if one exists.
//
// Use RequireAuth for routes that must be protected.
func LoadUser(userRepo *repo.UserRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := GetSessionUserID(r)
			if !ok {
				// No session — continue as anonymous
				next.ServeHTTP(w, r)
				return
			}

			user, err := userRepo.GetByID(userID)
			if err != nil {
				// Session exists but user not found
				// (deleted account, invalid cookie)
				// Clear the stale cookie and continue anonymous
				ClearSessionCookie(w)
				next.ServeHTTP(w, r)
				return
			}

			// User found — store in context
			r = SetUser(r, user)
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth is middleware that blocks unauthenticated requests.
//
// If no user is in the context (set by LoadUser), the request
// is redirected to the login page.
//
// Use this on routes that require a logged-in user:
//
//	r.Group(func(r chi.Router) {
//	    r.Use(auth.RequireAuth)
//	    r.Get("/invoices", handler)
//	})
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r)
		if user == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
