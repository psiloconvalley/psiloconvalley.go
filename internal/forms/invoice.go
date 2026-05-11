// internal/forms/invoice.go
package forms

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"psiloconvalley/internal/repo"
)

type InvoiceFormData struct {
	ClientID       *int64
	CompanyName    string
	CompanyEmail   string
	CompanyAddress string
	CompanyCity    string
	CompanyState   string
	CompanyZip     string
	CompanyCountry string

	ClientName    string
	ClientEmail   string
	ClientAddress string
	ClientCity    string
	ClientState   string
	ClientZip     string
	ClientCountry string

	InvoiceNumber  string
	IssueDate      time.Time
	DueDate        *time.Time
	TaxRateBps     int64
	Notes          string
	PaymentDetails string
	Currency       string

	Items []repo.InvoiceItem
}

func ParseInvoiceForm(r *http.Request) (*InvoiceFormData, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("could not parse form: %w", err)
	}

	f := &InvoiceFormData{
		CompanyName:    strings.TrimSpace(r.FormValue("company_name")),
		CompanyEmail:   strings.TrimSpace(r.FormValue("company_email")),
		CompanyAddress: strings.TrimSpace(r.FormValue("company_address")),
		CompanyCity:    strings.TrimSpace(r.FormValue("company_city")),
		CompanyState:   strings.TrimSpace(r.FormValue("company_state")),
		CompanyZip:     strings.TrimSpace(r.FormValue("company_zip")),
		CompanyCountry: strings.TrimSpace(r.FormValue("company_country")),

		ClientName:    strings.TrimSpace(r.FormValue("client_name")),
		ClientEmail:   strings.TrimSpace(r.FormValue("client_email")),
		ClientAddress: strings.TrimSpace(r.FormValue("client_address")),
		ClientCity:    strings.TrimSpace(r.FormValue("client_city")),
		ClientState:   strings.TrimSpace(r.FormValue("client_state")),
		ClientZip:     strings.TrimSpace(r.FormValue("client_zip")),
		ClientCountry: strings.TrimSpace(r.FormValue("client_country")),

		InvoiceNumber:  strings.TrimSpace(r.FormValue("invoice_number")),
		Notes:          strings.TrimSpace(r.FormValue("notes")),
		PaymentDetails: strings.TrimSpace(r.FormValue("payment_details")),
		TaxRateBps:     parseBasisPoints(r.FormValue("tax_rate")),
		Currency:       strings.TrimSpace(r.FormValue("currency")),
	}

	if f.Currency == "" {
		f.Currency = "USD"
	}

	if cid := strings.TrimSpace(r.FormValue("client_id")); cid != "" {
		if v, err := strconv.ParseInt(cid, 10, 64); err == nil && v > 0 {
			f.ClientID = &v
		}
	}

	f.IssueDate = parseDateOrDefault(r, "issue_date", time.Now())
	f.DueDate = parseOptionalDate(r, "due_date")

	items, err := parseLineItems(r)
	if err != nil {
		return nil, err
	}
	f.Items = items

	return f, nil
}

// Updated to support details[]
func parseLineItems(r *http.Request) ([]repo.InvoiceItem, error) {
	descs := formSlice(r, "description")
	details := formSlice(r, "details")
	qtys := formSlice(r, "quantity")
	prices := formSlice(r, "unit_price")

	n := max(len(descs), len(details), len(qtys), len(prices))

	var items []repo.InvoiceItem
	for i := 0; i < n; i++ {
		desc := safeIndex(descs, i)
		if desc == "" {
			continue
		}

		qty, err := requirePositiveFloat(safeIndex(qtys, i))
		if err != nil {
			return nil, fmt.Errorf("quantity required for item #%d (%s)", i+1, desc)
		}

		priceStr := safeIndex(prices, i)
		if priceStr == "" {
			return nil, fmt.Errorf("unit price required for item #%d (%s)", i+1, desc)
		}
		priceCents := parseCents(priceStr)
		if priceCents < 0 {
			return nil, fmt.Errorf("invalid unit price for item #%d (%s)", i+1, desc)
		}

		detail := safeIndex(details, i)
		fullDesc := desc
		if detail != "" {
			fullDesc += "\n" + detail
		}

		items = append(items, repo.InvoiceItem{
			Description:    fullDesc,
			Quantity:       qty,
			UnitPriceCents: priceCents,
		})
	}

	return items, nil
}

// Helper functions (all included)
func trimField(r *http.Request, key string) string {
	return strings.TrimSpace(r.FormValue(key))
}

func parseOptionalInt64(r *http.Request, key string) *int64 {
	raw := strings.TrimSpace(r.FormValue(key))
	if raw == "" {
		return nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return nil
	}
	return &v
}

func parseDateOrDefault(r *http.Request, key string, fallback time.Time) time.Time {
	raw := strings.TrimSpace(r.FormValue(key))
	if raw == "" {
		return fallback
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return fallback
	}
	return t
}

func parseOptionalDate(r *http.Request, key string) *time.Time {
	raw := strings.TrimSpace(r.FormValue(key))
	if raw == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil
	}
	return &t
}

func parseBasisPoints(val string) int64 {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0
	}
	return int64(math.Round(f * 100))
}

func parseCents(val string) int64 {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0
	}
	return int64(math.Round(f * 100))
}

func formSlice(r *http.Request, key string) []string {
	if vals := r.Form[key+"[]"]; len(vals) > 0 {
		return vals
	}
	return r.Form[key]
}

func safeIndex(slice []string, i int) string {
	if i >= len(slice) {
		return ""
	}
	return strings.TrimSpace(slice[i])
}

func requirePositiveFloat(s string) (float64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty value")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	if v <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return v, nil
}

func max(vals ...int) int {
	m := 0
	for _, v := range vals {
		if v > m {
			m = v
		}
	}
	return m
}
