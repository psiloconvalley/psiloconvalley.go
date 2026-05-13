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

// =====================================================================
// Form Data
// =====================================================================

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

// =====================================================================
// Validation Types
//
// FIX M2: Instead of returning a raw error and rendering a blank 400
// page, we return a ParseResult carrying both the partially-valid form
// data AND field-level errors. The handler re-renders the form with
// errors shown inline and all valid fields pre-populated.
//
// This is especially critical on mobile where re-typing a full invoice
// form after a validation failure is a significant friction point.
// =====================================================================

type FieldError struct {
	Field   string
	Message string
}

type ParseResult struct {
	Data   *InvoiceFormData
	Errors []FieldError
}

func (pr *ParseResult) Valid() bool {
	return len(pr.Errors) == 0
}

func (pr *ParseResult) AddError(field, message string) {
	pr.Errors = append(pr.Errors, FieldError{
		Field:   field,
		Message: message,
	})
}

// =====================================================================
// Primary Parse Entry Point
// =====================================================================

// ParseInvoiceForm parses and validates the invoice form submission.
//
// Return contract:
//   - (nil, error)     — malformed HTTP request (body too large, bad
//                        multipart). Caller should respond 400 immediately.
//   - (*result, nil)   — always returned on successful HTTP parse.
//                        Check result.Valid() before proceeding.
//                        If !Valid(), re-render the form with result.Errors.
func ParseInvoiceForm(r *http.Request) (*ParseResult, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("could not parse form: %w", err)
	}

	result := &ParseResult{
		Data: &InvoiceFormData{
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
		},
	}

	f := result.Data

	if f.Currency == "" {
		f.Currency = "USD"
	}

	// FIX D5: parseOptionalInt64 is now used here instead of the
	// inline duplicate block that previously existed alongside it.
	f.ClientID = parseOptionalInt64(r, "client_id")

	f.IssueDate = parseDateOrDefault(r, "issue_date", time.Now())
	f.DueDate = parseOptionalDate(r, "due_date")

	// Validate required fields
	if f.CompanyName == "" {
		result.AddError("company_name", "Company name is required")
	}

	// Parse line items — item-level errors collected into result.Errors
	items, itemErrors := parseLineItems(r)
	result.Errors = append(result.Errors, itemErrors...)
	f.Items = items

	// Must have at least one valid item to constitute a real invoice
	if len(f.Items) == 0 && len(itemErrors) == 0 {
		result.AddError("items", "At least one line item is required")
	}

	return result, nil
}

// =====================================================================
// Line Item Parsing
//
// FIX C5: description and details are stored as separate fields on
// repo.InvoiceItem, each mapping to its own DB column. The previous
// newline-concatenation hack is removed entirely, enabling clean
// round-trip editing without string-parsing heuristics.
//
// FIX H3: renamed from max() to sliceMaxLen() to avoid shadowing the
// Go 1.21 builtin max(), which has an incompatible signature and would
// cause a compile error on any Go version upgrade.
// =====================================================================

func parseLineItems(r *http.Request) ([]repo.InvoiceItem, []FieldError) {
	descs   := formSlice(r, "description")
	details := formSlice(r, "details")
	qtys    := formSlice(r, "quantity")
	prices  := formSlice(r, "unit_price")

	n := sliceMaxLen(len(descs), len(details), len(qtys), len(prices))

	var items  []repo.InvoiceItem
	var errs   []FieldError

	for i := 0; i < n; i++ {
		desc := safeIndex(descs, i)
		if desc == "" {
			// Skip blank rows — mobile forms often submit trailing empties
			continue
		}

		item := repo.InvoiceItem{
			Description: desc,
			Details:     safeIndex(details, i), // FIX C5: independent field
		}

		qty, err := requirePositiveFloat(safeIndex(qtys, i))
		if err != nil {
			errs = append(errs, FieldError{
				Field:   fmt.Sprintf("quantity[%d]", i),
				Message: fmt.Sprintf("Quantity required for item %d (%s)", i+1, desc),
			})
			continue
		}
		item.Quantity = qty

		// FIX H4: parseCents now returns (int64, error).
		// Negative prices are rejected at the source rather than silently
		// stored and discovered later as corrupt data.
		priceCents, err := parseCents(safeIndex(prices, i))
		if err != nil {
			errs = append(errs, FieldError{
				Field:   fmt.Sprintf("unit_price[%d]", i),
				Message: fmt.Sprintf("Invalid price for item %d (%s): %s", i+1, desc, err),
			})
			continue
		}
		item.UnitPriceCents = priceCents

		items = append(items, item)
	}

	return items, errs
}

// =====================================================================
// Private Helpers
// =====================================================================

// parseOptionalInt64 parses a positive integer from a named form field.
// Returns nil if the field is empty, zero, or non-numeric.
//
// FIX D5: this function previously existed but was never called —
// the client_id parsing block inlined the same logic. It is now the
// single implementation used by ParseInvoiceForm.
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

// parseCents converts a decimal string price to integer cents.
//
// FIX H4: previously returned int64 with no error, silently passing
// negative values to the caller. Now returns an explicit error so
// negative prices are rejected before reaching the database.
func parseCents(val string) (int64, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, fmt.Errorf("not a valid number: %s", val)
	}
	if f < 0 {
		return 0, fmt.Errorf("price cannot be negative")
	}
	return int64(math.Round(f * 100)), nil
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

// sliceMaxLen returns the largest of the given integers.
//
// FIX H3: renamed from max() which shadowed the Go 1.21 builtin.
// The builtin max() is generic over ordered types; our variadic int
// version would cause a compile error on Go 1.21+ if not renamed.
func sliceMaxLen(vals ...int) int {
	m := 0
	for _, v := range vals {
		if v > m {
			m = v
		}
	}
	return m
}
