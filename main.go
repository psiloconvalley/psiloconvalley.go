package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"

	"psiloconvalley/internal/repo"
)

// =====================================================================
// Application Container
// =====================================================================

// app owns all shared dependencies.
//
// This replaces package-level globals with a single struct that can be:
// → Passed to handlers explicitly
// → Injected with mocks in tests
// → Extended cleanly when auth/stripe land
type app struct {
	templates  *template.Template
	invRepo    *repo.InvoiceRepo
	clientRepo *repo.ClientRepo
	bizRepo    *repo.BusinessRepo
}

// =====================================================================
// Template Helpers
// =====================================================================

// money formats a float64 as a USD string.
func money(v float64) string {
	return fmt.Sprintf("$%.2f", v)
}

// field safely formats any value for use in an HTML input's value attr.
//
// Handles nil, pointer types, time.Time, and numeric zero suppression
// so templates never need to do unsafe type assertions themselves.
func field(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case *string:
		if val == nil {
			return ""
		}
		return *val
	case float64:
		if val == 0.0 {
			return ""
		}
		return fmt.Sprintf("%.2f", val)
	case *float64:
		if val == nil || *val == 0.0 {
			return ""
		}
		return fmt.Sprintf("%.2f", *val)
	case time.Time:
		if val.IsZero() {
			return ""
		}
		return val.Format("2006-01-02")
	case *time.Time:
		if val == nil || (*val).IsZero() {
			return ""
		}
		return (*val).Format("2006-01-02")
	default:
		return fmt.Sprintf("%v", val)
	}
}

// =====================================================================
// App Bootstrap
// =====================================================================

func newApp() *app {
	funcs := template.FuncMap{
		"money": money,
		"field": field,
	}

	t, err := template.New("").
		Funcs(funcs).
		ParseGlob("templates/*.tmpl")
	if err != nil {
		log.Fatal("template parse error:", err)
	}

	return &app{
		templates:  t,
		invRepo:    repo.NewInvoiceRepo(db),
		clientRepo: repo.NewClientRepo(db),
		bizRepo:    repo.NewBusinessRepo(db),
	}
}

// =====================================================================
// Rendering Helper
// =====================================================================

// render executes a named template and handles errors consistently.
func (a *app) render(w http.ResponseWriter, name string, data any) {
	if err := a.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template error [%s]: %v", name, err)
		http.Error(w, "Server error", http.StatusInternalServerError)
	}
}

// =====================================================================
// Form Parsing Helper
// =====================================================================

// invoiceFormData holds every field parsed from the invoice form.
//
// Extracting form parsing into its own struct means:
// → Create and Update share one parsing function
// → Adding a new field = change in ONE place only
// → Easy to validate as a unit before touching the DB
type invoiceFormData struct {
	ClientID       *int64
	CompanyName    string
	CompanyEmail   string
	CompanyAddress string
	CompanyCity    string
	CompanyZip     string
	CompanyState   string
	CompanyCountry string
	ClientName     string
	ClientEmail    string
	ClientAddress  string
	ClientCity     string
	ClientZip      string
	ClientState    string
	ClientCountry  string
	InvoiceNumber  string
	IssueDate      time.Time
	DueDate        *time.Time
	TaxRate        float64
	Notes          string
	PaymentDetails string
	Currency       string
	Items          []repo.InvoiceItem
}

// parseInvoiceForm reads and validates every field from an invoice form
// submission. Called by both create and update handlers so form parsing
// logic never drifts between the two paths.
//
// Takes http.ResponseWriter so MaxBytesReader can send a proper 413
// response if the payload exceeds the 1MB safety limit.
func parseInvoiceForm(w http.ResponseWriter, r *http.Request) (*invoiceFormData, error) {
	// Guard against absurdly large payloads before parsing
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB max

	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("could not parse form: %w", err)
	}

	f := &invoiceFormData{
		CompanyName:    r.FormValue("company_name"),
		CompanyEmail:   r.FormValue("company_email"),
		CompanyAddress: r.FormValue("company_address"),
		CompanyCity:    r.FormValue("company_city"),
		CompanyZip:     r.FormValue("company_zip"),
		CompanyState:   r.FormValue("company_state"),
		CompanyCountry: r.FormValue("company_country"),
		ClientName:     r.FormValue("client_name"),
		ClientEmail:    r.FormValue("client_email"),
		ClientAddress:  r.FormValue("client_address"),
		ClientCity:     r.FormValue("client_city"),
		ClientZip:      r.FormValue("client_zip"),
		ClientState:    r.FormValue("client_state"),
		ClientCountry:  r.FormValue("client_country"),
		InvoiceNumber:  r.FormValue("invoice_number"),
		Notes:          r.FormValue("notes"),
		PaymentDetails: r.FormValue("payment_details"),
	}

	// Optional client FK
	if cid := r.FormValue("client_id"); cid != "" {
		if v, err := strconv.ParseInt(cid, 10, 64); err == nil {
			f.ClientID = &v
		}
	}

	// Dates
	f.IssueDate = time.Now()
	if val := r.FormValue("issue_date"); val != "" {
		if t, err := time.Parse("2006-01-02", val); err == nil {
			f.IssueDate = t
		}
	}
	if val := r.FormValue("due_date"); val != "" {
		if t, err := time.Parse("2006-01-02", val); err == nil {
			f.DueDate = &t
		}
	}

	// Numerics
	f.TaxRate, _ = strconv.ParseFloat(r.FormValue("tax_rate"), 64)

	// Currency with sensible default
	f.Currency = r.FormValue("currency")
	if f.Currency == "" {
		f.Currency = "USD"
	}

	// Line items — parallel arrays from multi-value form fields
	descs := r.Form["description"]
	qtys := r.Form["quantity"]
	prices := r.Form["unit_price"]

	for i := range descs {
		if descs[i] == "" {
			continue
		}
		qty, _ := strconv.ParseFloat(qtys[i], 64)
		price, _ := strconv.ParseFloat(prices[i], 64)

		f.Items = append(f.Items, repo.InvoiceItem{
			Description: descs[i],
			Quantity:    qty,
			UnitPrice:   price,
		})
	}

	return f, nil
}

// =====================================================================
// Security Middleware
// =====================================================================

// securityHeaders adds baseline HTTP security headers to every response.
//
// These headers defend against:
// → Clickjacking           (X-Frame-Options)
// → MIME type sniffing      (X-Content-Type-Options)
// → Data leakage            (Referrer-Policy)
// → XSS in older browsers   (X-XSS-Protection)
// → Unauthorized API access  (Permissions-Policy)
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

// =====================================================================
// Handlers
// =====================================================================

func (a *app) healthHandler(w http.ResponseWriter, r *http.Request) {
	if err := db.Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintln(w, "db unavailable")
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

func (a *app) homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	a.render(w, "home.tmpl", nil)
}

func (a *app) invoiceNewHandler(w http.ResponseWriter, r *http.Request) {
	a.render(w, "invoice_new.tmpl", map[string]any{
		"Invoice":             &repo.Invoice{},
		"Items":               []repo.InvoiceItem{},
		"Mode":                "create",
		"SourceInvoiceNumber": "",
	})
}

func (a *app) invoiceCreateHandler(w http.ResponseWriter, r *http.Request) {
	f, err := parseInvoiceForm(w, r)
	if err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	// Auto-generate invoice number if not supplied
	if f.InvoiceNumber == "" {
		f.InvoiceNumber = fmt.Sprintf("INV-%d", time.Now().UnixNano()/1_000_000)
	}

	inv := &repo.Invoice{
		ClientID:       f.ClientID,
		CompanyName:    f.CompanyName,
		CompanyEmail:   f.CompanyEmail,
		CompanyAddress: f.CompanyAddress,
		CompanyCity:    f.CompanyCity,
		CompanyZip:     f.CompanyZip,
		CompanyState:   f.CompanyState,
		CompanyCountry: f.CompanyCountry,
		ClientName:     f.ClientName,
		ClientEmail:    f.ClientEmail,
		ClientAddress:  f.ClientAddress,
		ClientCity:     f.ClientCity,
		ClientZip:      f.ClientZip,
		ClientState:    f.ClientState,
		ClientCountry:  f.ClientCountry,
		InvoiceNumber:  f.InvoiceNumber,
		IssueDate:      f.IssueDate,
		DueDate:        f.DueDate,
		TaxRate:        f.TaxRate,
		Notes:          f.Notes,
		PaymentDetails: f.PaymentDetails,
		Currency:       f.Currency,
		Status:         "draft",
	}

	id, err := a.invRepo.CreateInvoice(inv, f.Items)
	if err != nil {
		log.Println("create invoice error:", err)
		http.Error(w, "could not create invoice", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/invoices/%d", id), http.StatusSeeOther)
}

func (a *app) invoiceDetailHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	inv, items, err := a.invRepo.GetInvoiceWithItems(id)
	if err != nil {
		log.Println("get invoice error:", err)
		http.NotFound(w, r)
		return
	}

	a.render(w, "invoice_detail.tmpl", map[string]any{
		"Invoice": inv,
		"Items":   items,
	})
}

func (a *app) invoiceDuplicateHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	inv, items, err := a.invRepo.GetInvoiceWithItems(id)
	if err != nil {
		log.Println("duplicate invoice error:", err)
		http.Error(w, "could not load invoice", http.StatusInternalServerError)
		return
	}

	sourceNumber := inv.InvoiceNumber

	// Reset identity — new invoice, new identity
	inv.ID = 0
	inv.InvoiceNumber = ""
	inv.Status = "draft"
	inv.IssueDate = time.Now()
	inv.DueDate = nil
	inv.CreatedAt = time.Time{}
	inv.UpdatedAt = time.Time{}

	a.render(w, "invoice_new.tmpl", map[string]any{
		"Invoice":             inv,
		"Items":               items,
		"Mode":                "duplicate",
		"SourceInvoiceNumber": sourceNumber,
	})
}

func (a *app) invoiceEditHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	inv, items, err := a.invRepo.GetInvoiceWithItems(id)
	if err != nil {
		log.Println("edit load error:", err)
		http.NotFound(w, r)
		return
	}

	// State machine gate — only drafts are editable
	if inv.Status != "draft" {
		log.Printf("blocked edit attempt: invoice %d status=%s", id, inv.Status)
		http.Error(w, "Only draft invoices can be edited", http.StatusForbidden)
		return
	}

	a.render(w, "invoice_new.tmpl", map[string]any{
		"Invoice":             inv,
		"Items":               items,
		"Mode":                "edit",
		"SourceInvoiceNumber": "",
	})
}

func (a *app) invoiceUpdateHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	// Load existing record — verify it exists and is still draft
	existing, _, err := a.invRepo.GetInvoiceWithItems(id)
	if err != nil {
		log.Println("update load error:", err)
		http.NotFound(w, r)
		return
	}
	if existing.Status != "draft" {
		http.Error(w, "Only draft invoices can be updated", http.StatusForbidden)
		return
	}

	f, err := parseInvoiceForm(w, r)
	if err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	// Invoice number is locked on edit — never let the form change it
	if f.InvoiceNumber == "" {
		f.InvoiceNumber = existing.InvoiceNumber
	}
	if f.Currency == "" {
		f.Currency = existing.Currency
	}

	inv := &repo.Invoice{
		ID:                id,
		BusinessProfileID: existing.BusinessProfileID,
		ClientID:          f.ClientID,
		CompanyName:       f.CompanyName,
		CompanyEmail:      f.CompanyEmail,
		CompanyAddress:    f.CompanyAddress,
		CompanyCity:       f.CompanyCity,
		CompanyZip:        f.CompanyZip,
		CompanyState:      f.CompanyState,
		CompanyCountry:    f.CompanyCountry,
		ClientName:        f.ClientName,
		ClientEmail:       f.ClientEmail,
		ClientAddress:     f.ClientAddress,
		ClientCity:        f.ClientCity,
		ClientZip:         f.ClientZip,
		ClientState:       f.ClientState,
		ClientCountry:     f.ClientCountry,
		InvoiceNumber:     f.InvoiceNumber,
		IssueDate:         f.IssueDate,
		DueDate:           f.DueDate,
		TaxRate:           f.TaxRate,
		Notes:             f.Notes,
		PaymentDetails:    f.PaymentDetails,
		Currency:          f.Currency,
		Status:            existing.Status,
	}

	if err := a.invRepo.UpdateInvoice(inv, f.Items); err != nil {
		log.Println("update invoice error:", err)
		http.Error(w, "could not update invoice", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/invoices/%d", id), http.StatusSeeOther)
}

func (a *app) invoicesListHandler(w http.ResponseWriter, r *http.Request) {
	list, err := a.invRepo.ListInvoices(50, 0)
	if err != nil {
		log.Println("list invoices error:", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	a.render(w, "invoices_list.tmpl", map[string]any{
		"Invoices": list,
	})
}

// =====================================================================
// Utility
// =====================================================================

// parseID extracts and validates the {id} URL parameter from Chi routes.
func parseID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// =====================================================================
// Entry Point
// =====================================================================

func main() {
	_ = godotenv.Load()

	initDB()
	defer db.Close()

	a := newApp()

	r := chi.NewRouter()

	// Security middleware — runs on every request
	r.Use(securityHeaders)

	// Static assets
	fs := http.FileServer(http.Dir("./static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	// System routes
	r.Get("/", a.homeHandler)
	r.Get("/healthz", a.healthHandler)

	// Invoice routes — order matters: specific paths before catch-all
	r.Route("/invoices", func(r chi.Router) {
		r.Get("/", a.invoicesListHandler)
		r.Get("/new", a.invoiceNewHandler)
		r.Post("/create", a.invoiceCreateHandler)
		r.Get("/{id}/duplicate", a.invoiceDuplicateHandler)
		r.Get("/{id}/edit", a.invoiceEditHandler)
		r.Post("/{id}/edit", a.invoiceUpdateHandler)
		r.Get("/{id}", a.invoiceDetailHandler)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Production-grade server with explicit timeouts.
	// Prevents slow-client denial-of-service attacks.
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("server starting on :%s", port)
	log.Fatal(srv.ListenAndServe())
}
