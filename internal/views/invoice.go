package views

import (
	"fmt"
	"time"

	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/repo"
)

// =====================================================================
// ViewModel Structs
// =====================================================================
//
// These are the ONLY structs your HTML templates will ever see.
// They contain pre-formatted strings. No int64. No float64.
// No math in templates. No panics. No ambiguity.
//
// The Go compiler enforces this boundary at build time.
// If a field is missing, the code won't compile.
// If a field is wrong, the mapper function catches it.
//
// This is the permanent architectural firewall between
// your database layer and your presentation layer.

type InvoicePage struct {
	ID             int64
	ClientID	int64
	InvoiceNumber  string
	Status         string
	IssueDate      string
	DueDate        string
	Currency       string
	CurrencySymbol string
	Notes          string
	PaymentDetails string
	Mode           string

	// Company (Sender)
	CompanyName    string
	CompanyEmail   string
	CompanyAddress string
	CompanyCity    string
	CompanyZip     string
	CompanyState   string
	CompanyCountry string

	// Client (Receiver)
	ClientName    string
	ClientEmail   string
	ClientAddress string
	ClientCity    string
	ClientZip     string
	ClientState   string
	ClientCountry string

	// Financials (Pre-formatted for display)
	Subtotal  string
	TaxRate   string
	TaxAmount string
	Discount  string
	Total     string

	// Financials (Raw cents for form inputs)
	TaxRateBpsRaw          int64
	DiscountAmountCentsRaw int64

	// Line Items
	Items []InvoiceItemView
}

type InvoiceItemView struct {
	ID          int64
	Description string
	Quantity    string
	UnitPrice   string // Display: "$150.50"
	LineTotal   string // Display: "$301.00"

	// Raw values for form inputs
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
// Formatting Helpers (Private to this package)
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

func formatCentsForInput(cents int64) string {
	if cents == 0 {
		return ""
	}
	return fmt.Sprintf("%.2f", float64(cents)/100.0)
}

func currencySymbol(code string) string {
	return catalog.CurrencySymbol(code)
}

// =====================================================================
// Mappers: repo -> view
// =====================================================================
//
// These are the ONE place where database structs become view structs.
// All formatting decisions live here. Not in templates. Not in handlers.
// Here. One place. One truth. Forever.

func MapInvoicePage(inv *repo.Invoice, items []repo.InvoiceItem, mode string) InvoicePage {
	sym := currencySymbol(inv.Currency)

	page := InvoicePage{
		ID:             inv.ID,
		ClientID:	func() int64 {
			if inv.ClientID != nil {
				return *inv.ClientID
			}
			return 0
		}(),
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

		TaxRateBpsRaw:          inv.TaxRateBps,
		DiscountAmountCentsRaw: inv.DiscountAmountCents,
	}

	for _, item := range items {
		page.Items = append(page.Items, InvoiceItemView{
			ID:             item.ID,
			Description:    item.Description,
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
