// internal/views/invoice.go
package views

import (
	"fmt"
	"time"

	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/repo"
)

// =====================================================================
// View Models
// =====================================================================

type InvoicePage struct {
	ID             int64
	ClientID       int64
	InvoiceNumber  string
	Status         string
	IssueDate      string
	DueDate        string
	Currency       string
	CurrencySymbol string
	Notes          string
	PaymentDetails string
	Mode           string

	// Company
	CompanyName    string
	CompanyEmail   string
	CompanyAddress string
	CompanyCity    string
	CompanyZip     string
	CompanyState   string
	CompanyCountry string

	// Client
	ClientName    string
	ClientEmail   string
	ClientAddress string
	ClientCity    string
	ClientZip     string
	ClientState   string
	ClientCountry string

	// Financials (formatted)
	Subtotal  string
	TaxRate   string
	TaxAmount string
	Discount  string
	Total     string

	TaxRateBps          int64
	DiscountAmountCentsRaw int64

	Items []InvoiceItemView
}

type InvoiceItemView struct {
	Description string
	Details     string   // ← NEW: Additional details per line item
	Quantity    string
	UnitPrice   string
	LineTotal   string

	QuantityRaw    float64
	UnitPriceCents int64
}

type InvoiceListRow struct {
	ID            int64
	InvoiceNumber string
	ClientName    string
	Status        string
	IssueDate     string
	DueDate       string
	Total         string
}

// =====================================================================
// Formatting Helpers
// =====================================================================

func formatMoney(cents int64, symbol string) string {
	if symbol == "" {
		symbol = "$"
	}
	return fmt.Sprintf("%s%.2f", symbol, float64(cents)/100.0)
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func formatDatePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func formatPercent(bps int64) string {
	if bps == 0 {
		return "0"
	}
	return fmt.Sprintf("%.2f", float64(bps)/100.0)
}

func formatQuantity(q float64) string {
	if q == float64(int64(q)) {
		return fmt.Sprintf("%.0f", q)
	}
	return fmt.Sprintf("%.2f", q)
}

func currencySymbol(code string) string {
	return catalog.CurrencySymbol(code)
}

func ptrToInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// =====================================================================
// Mappers
// =====================================================================

func MapInvoicePage(inv *repo.Invoice, items []repo.InvoiceItem, mode string) InvoicePage {
	sym := currencySymbol(inv.Currency)

	page := InvoicePage{
		ID:             inv.ID,
		ClientID:       ptrToInt64(inv.ClientID),
		InvoiceNumber:  inv.InvoiceNumber,
		Status:         inv.Status,
		IssueDate:      formatDate(inv.IssueDate),
		DueDate:        formatDatePtr(inv.DueDate),
		Currency:       inv.Currency,
		CurrencySymbol: sym,
		Notes:          inv.Notes,
		PaymentDetails: inv.PaymentDetails,
		Mode:           mode,

		CompanyName:    inv.CompanyName,
		CompanyEmail:   inv.CompanyEmail,
		CompanyAddress: inv.CompanyAddress,
		CompanyCity:    inv.CompanyCity,
		CompanyZip:     inv.CompanyZip,
		CompanyState:   inv.CompanyState,
		CompanyCountry: inv.CompanyCountry,

		ClientName:    inv.ClientName,
		ClientEmail:   inv.ClientEmail,
		ClientAddress: inv.ClientAddress,
		ClientCity:    inv.ClientCity,
		ClientZip:     inv.ClientZip,
		ClientState:   inv.ClientState,
		ClientCountry: inv.ClientCountry,

		Subtotal:  formatMoney(inv.SubtotalCents, sym),
		TaxRate:   formatPercent(inv.TaxRateBps),
		TaxAmount: formatMoney(inv.TaxAmountCents, sym),
		Discount:  formatMoney(inv.DiscountAmountCents, sym),
		Total:     formatMoney(inv.TotalCents, sym),

		TaxRateBps:             inv.TaxRateBps,
		DiscountAmountCentsRaw: inv.DiscountAmountCents,
	}

	for _, item := range items {
		page.Items = append(page.Items, InvoiceItemView{
			Description:    item.Description,
			Details:        "", // Will be populated when we add a Details column to the DB
			Quantity:       formatQuantity(item.Quantity),
			UnitPrice:      formatMoney(item.UnitPriceCents, sym),
			LineTotal:      formatMoney(item.LineTotalCents, sym),
			QuantityRaw:    item.Quantity,
			UnitPriceCents: item.UnitPriceCents,
		})
	}

	return page
}

func MapInvoiceList(invoices []repo.Invoice) []InvoiceListRow {
	rows := make([]InvoiceListRow, 0, len(invoices))
	for _, inv := range invoices {
		sym := currencySymbol(inv.Currency)
		rows = append(rows, InvoiceListRow{
			ID:            inv.ID,
			InvoiceNumber: inv.InvoiceNumber,
			ClientName:    inv.ClientName,
			Status:        inv.Status,
			IssueDate:     formatDate(inv.IssueDate),
			DueDate:       formatDatePtr(inv.DueDate),
			Total:         formatMoney(inv.TotalCents, sym),
		})
	}
	return rows
}
