// internal/audit/audit.go
package audit

import (
	"context"
	"net/http"

	"psiloconvalley/internal/repo"
)

// Action constants — single source of truth for all audit action names.
// Format: domain.verb
// Never use raw strings in handlers — always use these constants.
const (
	// Invoice actions
	ActionInvoiceCreated      = "invoice.created"
	ActionInvoiceUpdated      = "invoice.updated"
	ActionInvoiceDeleted      = "invoice.deleted"
	ActionInvoiceStatusChanged = "invoice.status_changed"
	ActionInvoiceSent         = "invoice.sent"
	ActionInvoiceDuplicated   = "invoice.duplicated"

	// Estimate actions
	ActionEstimateCreated      = "estimate.created"
	ActionEstimateUpdated      = "estimate.updated"
	ActionEstimateDeleted      = "estimate.deleted"
	ActionEstimateStatusChanged = "estimate.status_changed"
	ActionEstimateSent         = "estimate.sent"
	ActionEstimateConverted    = "estimate.converted"
	ActionEstimateResponded    = "estimate.responded"

	// Auth actions
	ActionAuthLogin       = "auth.login"
	ActionAuthLoginFailed = "auth.login_failed"
	ActionAuthLogout      = "auth.logout"
	ActionAuthRegister    = "auth.register"

	// Payment actions
	ActionPaymentReceived = "payment.received"
	ActionPaymentFailed   = "payment.failed"
)

// Entity type constants — what kind of record was affected.
const (
	EntityInvoice  = "invoice"
	EntityEstimate = "estimate"
	EntityUser     = "user"
	EntityPayment  = "payment"
)

// Entry holds the data for a single audit log event.
type Entry struct {
	UserID     *int64
	Action     string
	EntityType string
	EntityID   *int64
	IPAddress  string
	Metadata   map[string]any
}

// Log writes an audit entry. Errors are swallowed — audit logging must
// never block or fail a user action. All failures are logged internally
// by the repo layer.
func Log(ctx context.Context, auditRepo *repo.AuditRepo, e Entry) {
	auditRepo.Log(ctx, repo.AuditLog{
		UserID:     e.UserID,
		Action:     e.Action,
		EntityType: e.EntityType,
		EntityID:   e.EntityID,
		IPAddress:  e.IPAddress,
		Metadata:   e.Metadata,
	})
}

// IPFromRequest extracts the real client IP address from a request.
// Checks X-Forwarded-For first (set by Railway/proxies), then RemoteAddr.
func IPFromRequest(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}

// UserIDPtr is a convenience helper to get a *int64 from a user.
// Returns nil if user is nil — safe for anonymous actions.
func UserIDPtr(userID int64) *int64 {
	return &userID
}

// EntityIDPtr converts an int64 to *int64 for entity ID fields.
func EntityIDPtr(id int64) *int64 {
	return &id
}
