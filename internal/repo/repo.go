// internal/repo/repo.go
package repo

import (
	"context"
	"database/sql"
	"math"
	"time"
)

// =====================================================================
// InvoiceRepo — concrete PostgreSQL implementation
// =====================================================================

type InvoiceRepo struct{ db *sql.DB }

func NewInvoiceRepo(db *sql.DB) *InvoiceRepo { return &InvoiceRepo{db: db} }

// calculateTotals computes line totals, subtotal, tax, discount, and grand total.
// Called by CreateInvoice and UpdateInvoice before writing to the database.
func calculateTotals(inv *Invoice, items []InvoiceItem) []InvoiceItem {
	var subtotalCents int64
	for i := range items {
		lineCents := int64(math.Round(
			items[i].Quantity * float64(items[i].UnitPriceCents),
		))
		items[i].LineTotalCents = lineCents
		subtotalCents += lineCents
	}

	taxCents := (subtotalCents * inv.TaxRateBps) / 10000
	preTaxTotal := subtotalCents + taxCents

	if inv.DiscountAmountCents > preTaxTotal {
		inv.DiscountAmountCents = preTaxTotal
	}
	if inv.DiscountAmountCents < 0 {
		inv.DiscountAmountCents = 0
	}

	inv.SubtotalCents = subtotalCents
	inv.TaxAmountCents = taxCents
	inv.TotalCents = preTaxTotal - inv.DiscountAmountCents
	return items
}

// =====================================================================
// InvoiceStore — interface for all invoice operations.
//
// Why this exists:
// *InvoiceRepo is a concrete type tied to a real PostgreSQL database.
// Any handler that uses *InvoiceRepo directly cannot be tested without
// a live database. By depending on this interface instead, handlers
// become testable with a fake implementation — no database required.
//
// The real *InvoiceRepo satisfies this interface automatically.
// Go interfaces are implicit — nothing in InvoiceRepo needs to change.
// =====================================================================

type InvoiceStore interface {
	CreateWithToken(
		ctx context.Context,
		user *User,
		anonymousToken string,
		clientName string,
		amount float64,
		description string,
	) (int64, error)

	CreateInvoice(
		ctx context.Context,
		inv *Invoice,
		items []InvoiceItem,
		anonymousToken string,
	) (int64, error)

	GetInvoiceWithItems(
		ctx context.Context,
		id int64,
	) (*Invoice, []InvoiceItem, error)

	ListInvoices(
		ctx context.Context,
		limit, offset int,
		userID *int64,
	) ([]Invoice, error)

	ListEstimates(
		ctx context.Context,
		limit, offset int,
		userID int64,
	) ([]Invoice, error)

	InvoiceNumberExists(
		ctx context.Context,
		number string,
		userID int64,
	) (bool, error)

	UpdateInvoice(
		ctx context.Context,
		inv *Invoice,
		items []InvoiceItem,
	) error

	UpdateInvoiceStatus(
		ctx context.Context,
		id int64,
		newStatus string,
		userID int64,
	) error

	DeleteDraftInvoice(
		ctx context.Context,
		id int64,
		userID int64,
	) error

	EnsurePublicToken(
		ctx context.Context,
		invoiceID int64,
	) (string, error)

	GetDashboardStats(
		ctx context.Context,
		userID int64,
	) (*DashboardStats, error)

	GetAdminStats(
		ctx context.Context,
		db *sql.DB,
	) (*AdminStats, error)

	ListInvoicesForReport(
		ctx context.Context,
		userID int64,
		start, end time.Time,
		status string,
	) ([]InvoiceReportRow, error)

	GetClientScorecards(
		ctx context.Context,
		userID int64,
	) ([]ClientScorecard, error)
}
