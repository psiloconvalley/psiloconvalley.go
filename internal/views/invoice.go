// internal/views/invoice.go
package views

import (
	"fmt"
	"time"

	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/repo"
)

// =====================================================================
// Render Hints
//
// FIX M3: RenderHints carries request-context signals from the handler
// to the template. This is the clean alternative to duplicating structs
// or embedding request objects in view models.
//
// The handler sets these fields based on request characteristics
// (query params, user agent, etc.). Templates read them with {{if .Hints.PDFMode}}
// to conditionally suppress navigation, buttons, or switch layouts.
//
// Why not user-agent sniffing? UA strings are unreliable and change
// with every browser update. Query params (?mode=inline, ?layout=mobile)
// are explicit, testable, and cache-friendly.
// =====================================================================

type RenderHints struct {
	PDFMode       bool // true when rendering for PDF generation — hides nav/buttons
	InlinePDF     bool // true when PDF should be served inline vs attachment
	MobileLayout  bool // true when mobile-optimised template variant is preferred
}

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
	LogoURL        string

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

	// Financials (formatted for display)
	Subtotal  string
	TaxRate   string
	TaxAmount string
	Discount  string
	Total     string

	// Raw values for form re-population
	TaxRateBps             int64
	DiscountAmountCentsRaw int64

	Items []InvoiceItemView

	// FIX M3: Populated by the handler, read by templates.
	Hints RenderHints
	ShowLogo  bool
	ShowTitle bool
}

// FIX H5 / C5: Details is now populated from repo.InvoiceItem.Details,
// which maps to the invoice_items.details DB column added in the migration.
// The Details field is no longer always empty string.
type InvoiceItemView struct {
	Description string
	Details     string  // ← now correctly populated
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
// Formatting Helpers — unchanged, all correct
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

// FIX D6: renamed local variable from 'page' to 'vw' (view).
// 'page' shadowed any future import of a package named 'page'
// and was semantically confusing given this function returns InvoicePage.
func MapInvoicePage(inv *repo.Invoice, items []repo.InvoiceItem, mode string) InvoicePage {
	sym := currencySymbol(inv.Currency)

	vw := InvoicePage{
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
		LogoURL:        inv.LogoURL,

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
		ShowLogo:  inv.ShowLogo,
		ShowTitle: inv.ShowTitle,

		// Hints is zero-value (all false) by default.
		// The handler sets fields on this after calling MapInvoicePage.
	}

	for _, item := range items {
		vw.Items = append(vw.Items, InvoiceItemView{
			Description:    item.Description,
			Details:        item.Details, // FIX H5: no longer hardcoded ""
			Quantity:       formatQuantity(item.Quantity),
			UnitPrice:      formatMoney(item.UnitPriceCents, sym),
			LineTotal:      formatMoney(item.LineTotalCents, sym),
			QuantityRaw:    item.Quantity,
			UnitPriceCents: item.UnitPriceCents,
		})
	}

	return vw
}

func MapInvoiceList(invoices []repo.Invoice) []InvoiceListRow {
	rows := make([]InvoiceListRow, 0, len(invoices))
	for _, inv := range invoices {
		sym := currencySymbol(inv.Currency) // FIX C2: currency now present from ListInvoices
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
