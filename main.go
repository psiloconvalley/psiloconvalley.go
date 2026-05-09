package main

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"github.com/justinas/nosurf"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/pdf"
	"psiloconvalley/internal/repo"
	"psiloconvalley/internal/views"
)

// =====================================================================
// Application Container
// =====================================================================

type app struct {
	templates  *template.Template
	invRepo    *repo.InvoiceRepo
	clientRepo *repo.ClientRepo
	bizRepo    *repo.BusinessRepo
	userRepo   *repo.UserRepo
}

// =====================================================================
// Helpers
// =====================================================================

func money(cents int64) string {
	return fmt.Sprintf("$%.2f", float64(cents)/100.0)
}

func formatCentsForInput(cents int64) string {
	if cents == 0 {
		return ""
	}
	return fmt.Sprintf("%.2f", float64(cents)/100.0)
}

func bpsToPercent(bps int64) string {
	if bps == 0 {
		return ""
	}
	return fmt.Sprintf("%.2f", float64(bps)/100.0)
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

func parseBps(val string) int64 {
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
	case int64:
		if val == 0 {
			return ""
		}
		return fmt.Sprintf("%d", val)
	case float64:
		if val == 0.0 {
			return ""
		}
		return fmt.Sprintf("%.2f", val)
	case time.Time:
		if val.IsZero() {
			return ""
		}
		return val.Format("2006-01-02")
	default:
		return fmt.Sprintf("%v", val)
	}
}

// =====================================================================
// Usage Limits
// =====================================================================

const freePlanMonthlyLimit = 100

func (a *app) hasReachedLimit(r *http.Request) bool {
	user := auth.GetUser(r)
	if user == nil {
		return auth.AnonLimitReached(r)
	}
	if user.Plan == "pro" {
		return false
	}
	count, err := a.userRepo.GetMonthlyInvoiceCount(r.Context(), user.ID)
	if err != nil {
		log.Printf("usage limit check error: %v", err)
		return false
	}
	return count >= freePlanMonthlyLimit
}

// =====================================================================
// App Constructor
// =====================================================================

func newApp() *app {
	funcs := template.FuncMap{
		"money":        money,
		"field":        field,
		"formatCents":  formatCentsForInput,
		"bpsToPercent": bpsToPercent,
	}

	t, err := template.New("").Funcs(funcs).ParseFS(templateFiles, "templates/*.tmpl")
	if err != nil {
		log.Fatal("template parse error:", err)
	}

	return &app{
		templates:  t,
		invRepo:    repo.NewInvoiceRepo(db),
		clientRepo: repo.NewClientRepo(db),
		bizRepo:    repo.NewBusinessRepo(db),
		userRepo:   repo.NewUserRepo(db),
	}
}

// =====================================================================
// Rendering
// =====================================================================

func (a *app) render(w http.ResponseWriter, r *http.Request, name string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	data["User"] = auth.GetUser(r)
	data["GoogleEnabled"] = auth.GoogleOAuthEnabled()
	data["csrfField"] = template.HTML(
		fmt.Sprintf(`<input type="hidden" name="csrf_token" value="%s">`, nosurf.Token(r)),
	)

	var buf bytes.Buffer
	if err := a.templates.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("template error [%s]: %v", name, err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

// =====================================================================
// Security
// =====================================================================

func canAccessInvoice(r *http.Request, inv *repo.Invoice) bool {
	if inv.UserID == nil {
		return true
	}
	user := auth.GetUser(r)
	if user == nil {
		return false
	}
	return user.ID == *inv.UserID
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; script-src 'self' 'unsafe-inline'; img-src 'self' data:;",
		)
		next.ServeHTTP(w, r)
	})
}

// =====================================================================
// Handlers: Platform
// =====================================================================

func (a *app) indexHandler(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "index.tmpl", nil)
}

func (a *app) toolsHubHandler(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "tools_hub.tmpl", nil)
}

func (a *app) healthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "ok")
}

func (a *app) feedbackHandler(w http.ResponseWriter, r *http.Request) {
	report := r.FormValue("report")
	user := auth.GetUser(r)
	uEmail := "Anonymous"
	if user != nil {
		uEmail = user.Email
	}
	log.Printf("\n[ANOMALY REPORT] %s\nFROM: %s\nCONTENT: %s\n-------------------",
		time.Now().Format("2006-01-02 15:04:05"), uEmail, report)
	http.Redirect(w, r, "/tools?transmitted=true", http.StatusSeeOther)
}

func (a *app) invoicePortalHandler(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "home.tmpl", nil)
}

// =====================================================================
// Handlers: Auth
// =====================================================================

func (a *app) registerGetHandler(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "register.tmpl", nil)
}

func (a *app) registerPostHandler(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	pass := r.FormValue("password")
	confirm := r.FormValue("confirm_password")

	log.Printf("REGISTER ATTEMPT: %s from %s", email, r.RemoteAddr)

	if email == "" || !strings.Contains(email, "@") {
		a.render(w, r, "register.tmpl", map[string]any{
			"Error": "Please enter a valid email address",
			"Email": email,
		})
		return
	}

	if len(pass) < 8 {
		a.render(w, r, "register.tmpl", map[string]any{
			"Error": "Password must be at least 8 characters",
			"Email": email,
		})
		return
	}

	if pass != confirm {
		a.render(w, r, "register.tmpl", map[string]any{
			"Error": "Passwords do not match",
			"Email": email,
		})
		return
	}

	id, err := a.userRepo.Create(email, pass)
	if err != nil {
		a.render(w, r, "register.tmpl", map[string]any{
			"Error": "An account with this email already exists",
			"Email": email,
		})
		return
	}
	auth.SetSessionCookie(w, id)
	http.Redirect(w, r, "/profile?welcome=true", http.StatusSeeOther)
}

func (a *app) loginGetHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{}

	switch r.URL.Query().Get("error") {
	case "oauth_failed":
		data["Error"] = "Google sign-in failed. Please try again or use email."
	case "oauth_denied":
		data["Error"] = "Google sign-in was cancelled."
	}

	if r.URL.Query().Get("reason") == "limit" {
		data["LimitBanner"] = true
	}

	a.render(w, r, "login.tmpl", data)
}

func (a *app) loginPostHandler(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	pass := r.FormValue("password")

	log.Printf("LOGIN ATTEMPT: %s from %s", email, r.RemoteAddr)

	user, err := a.userRepo.GetByEmail(email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			a.render(w, r, "login.tmpl", map[string]any{"Error": "Invalid credentials", "Email": email})
			return
		}
		log.Printf("login db error: %v", err)
		http.Error(w, "Auth service unavailable", http.StatusInternalServerError)
		return
	}
	if !user.CheckPassword(pass) {
		a.render(w, r, "login.tmpl", map[string]any{"Error": "Invalid credentials", "Email": email})
		return
	}
	auth.SetSessionCookie(w, user.ID)
	http.Redirect(w, r, "/tools", http.StatusSeeOther)
}

func (a *app) logoutHandler(w http.ResponseWriter, r *http.Request) {
	auth.ClearSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// =====================================================================
// Handlers: Google OAuth
// =====================================================================

func (a *app) googleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	googleUser, err := auth.ProcessGoogleCallback(w, r)
	if err != nil {
		log.Printf("google oauth callback error: %v", err)

		// State mismatch — retry instead of showing error
		if strings.Contains(err.Error(), "invalid oauth state") {
			log.Printf("OAuth state mismatch — redirecting to retry")
			http.Redirect(w, r, "/auth/google", http.StatusTemporaryRedirect)
			return
		}

		errMsg := "oauth_failed"
		if strings.Contains(err.Error(), "denied") {
			errMsg = "oauth_denied"
		}
		http.Redirect(w, r, "/login?error="+errMsg, http.StatusSeeOther)
		return
	}

	user, isNew, err := a.userRepo.FindOrCreateGoogleUser(
		googleUser.Email,
		googleUser.ID,
		googleUser.Name,
		googleUser.Picture,
	)
	if err != nil {
		log.Printf("google user upsert error: %v", err)
		http.Error(w, "Could not create or find account", http.StatusInternalServerError)
		return
	}

	auth.SetSessionCookie(w, user.ID)
	log.Printf("GOOGLE AUTH: %s (id=%d, new=%v)", user.Email, user.ID, isNew)

	if isNew {
		http.Redirect(w, r, "/profile?welcome=true", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/invoices", http.StatusSeeOther)
}

// =====================================================================
// Handlers: Business Profile
// =====================================================================

func (a *app) profileGetHandler(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	profile, err := a.bizRepo.GetByUserID(r.Context(), user.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("profile fetch error: %v", err)
		http.Error(w, "Could not load profile", http.StatusInternalServerError)
		return
	}

	a.render(w, r, "profile.tmpl", map[string]any{
		"Profile":    profile,
		"Saved":      r.URL.Query().Get("saved") == "true",
		"Welcome":    r.URL.Query().Get("welcome") == "true",
		"Currencies": catalog.SupportedCurrencies,
	})
}

func (a *app) profilePostHandler(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	p := &repo.BusinessProfile{
		UserID:  user.ID,
		Name:    strings.TrimSpace(r.FormValue("name")),
		Email:   strings.TrimSpace(r.FormValue("email")),
		Address: strings.TrimSpace(r.FormValue("address")),
		City:    strings.TrimSpace(r.FormValue("city")),
		State:   strings.TrimSpace(r.FormValue("state")),
		Zip:     strings.TrimSpace(r.FormValue("zip")),
		Country: strings.TrimSpace(r.FormValue("country")),
		TaxID:   strings.TrimSpace(r.FormValue("tax_id")),
		Currency: func() string {
			c := r.FormValue("currency")
			if c == "" {
				return "USD"
			}
			return c
		}(),
	}

	if p.Name == "" {
		a.render(w, r, "profile.tmpl", map[string]any{
			"Profile":    p,
			"Error":      "Company name is required",
			"Currencies": catalog.SupportedCurrencies,
		})
		return
	}

	if err := a.bizRepo.Upsert(r.Context(), p); err != nil {
		log.Printf("profile upsert error: %v", err)
		a.render(w, r, "profile.tmpl", map[string]any{
			"Profile":    p,
			"Error":      "Could not save profile",
			"Currencies": catalog.SupportedCurrencies,
		})
		return
	}

	http.Redirect(w, r, "/profile?saved=true", http.StatusSeeOther)
}

// =====================================================================
// Handlers: Invoices
// =====================================================================

func (a *app) invoicesListHandler(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	var uid *int64
	if user != nil {
		uid = &user.ID
	}

	list, err := a.invRepo.ListInvoices(r.Context(), 50, 0, uid)
	if err != nil {
		log.Printf("list invoices error: %v", err)
		http.Error(w, "Could not load invoices", http.StatusInternalServerError)
		return
	}

	rows := views.MapInvoiceList(list)
	a.render(w, r, "invoices_list.tmpl", map[string]any{"Invoices": rows})
}

func (a *app) invoiceDetailHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	inv, items, err := a.invRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || !canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	invoiceView := views.MapInvoicePage(inv, items, "view")
	a.render(w, r, "invoice_detail.tmpl", map[string]any{"Invoice": invoiceView})
}

func (a *app) invoiceNewHandler(w http.ResponseWriter, r *http.Request) {
	if a.hasReachedLimit(r) {
		http.Redirect(w, r, "/register?reason=limit", http.StatusSeeOther)
		return
	}

	inv := &repo.Invoice{
		CompanyCountry: catalog.DefaultCompanyCountry,
		CompanyState:   catalog.DefaultCompanyState,
		ClientCountry:  catalog.DefaultClientCountry,
		ClientState:    catalog.DefaultClientState,
		Currency:       catalog.DefaultCurrency,
		Status:         "draft",
		IssueDate:      time.Now(),
	}

	user := auth.GetUser(r)
	if user != nil {
		profile, err := a.bizRepo.GetByUserID(r.Context(), user.ID)
		if err == nil && profile != nil {
			inv.CompanyName = profile.Name
			inv.CompanyEmail = profile.Email
			inv.CompanyAddress = profile.Address
			inv.CompanyCity = profile.City
			inv.CompanyState = profile.State
			inv.CompanyZip = profile.Zip
			inv.CompanyCountry = profile.Country
			if profile.Currency != "" {
				inv.Currency = profile.Currency
			}
		}
	}

	invoiceView := views.MapInvoicePage(inv, nil, "create")
	a.render(w, r, "invoice_new.tmpl", map[string]any{
		"Invoice":    invoiceView,
		"Mode":       "create",
		"Currencies": catalog.SupportedCurrencies,
	})
}

func (a *app) invoiceCreateHandler(w http.ResponseWriter, r *http.Request) {
	if a.hasReachedLimit(r) {
		http.Redirect(w, r, "/register?reason=limit", http.StatusSeeOther)
		return
	}

	f, err := parseInvoiceForm(w, r)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if f.InvoiceNumber == "" {
		f.InvoiceNumber = fmt.Sprintf("INV-%d", time.Now().UnixNano()/1_000_000)
	}

	user := auth.GetUser(r)
	var uid *int64
	if user != nil {
		uid = &user.ID
	}

	inv := &repo.Invoice{
		UserID:         uid,
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
		TaxRateBps:     f.TaxRateBps,
		Notes:          f.Notes,
		PaymentDetails: f.PaymentDetails,
		Currency:       f.Currency,
		Status:         "draft",
	}

	id, err := a.invRepo.CreateInvoice(r.Context(), inv, f.Items)
	if err != nil {
		log.Printf("create invoice error: %v", err)
		http.Error(w, "Save failed", http.StatusInternalServerError)
		return
	}

	if user == nil {
		auth.SetAnonInvoiceCount(w, auth.GetAnonInvoiceCount(r)+1)
	}

	http.Redirect(w, r, fmt.Sprintf("/invoices/%d", id), http.StatusSeeOther)
}

func (a *app) invoiceEditHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	inv, items, err := a.invRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || !canAccessInvoice(r, inv) || inv.Status != "draft" {
		http.NotFound(w, r)
		return
	}

	invoiceView := views.MapInvoicePage(inv, items, "edit")
	a.render(w, r, "invoice_new.tmpl", map[string]any{
		"Invoice":    invoiceView,
		"Mode":       "edit",
		"Currencies": catalog.SupportedCurrencies,
	})
}

func (a *app) invoiceUpdateHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	ex, _, err := a.invRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || !canAccessInvoice(r, ex) {
		http.NotFound(w, r)
		return
	}

	f, err := parseInvoiceForm(w, r)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	inv := &repo.Invoice{
		ID:             id,
		UserID:         ex.UserID,
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
		InvoiceNumber:  ex.InvoiceNumber,
		IssueDate:      f.IssueDate,
		DueDate:        f.DueDate,
		TaxRateBps:     f.TaxRateBps,
		Notes:          f.Notes,
		PaymentDetails: f.PaymentDetails,
		Currency:       f.Currency,
		Status:         ex.Status,
	}

	if err := a.invRepo.UpdateInvoice(r.Context(), inv, f.Items); err != nil {
		log.Printf("update invoice error: %v", err)
		http.Error(w, "Update failed", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/invoices/%d", id), http.StatusSeeOther)
}

func (a *app) invoiceDuplicateHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	inv, items, err := a.invRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || !canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	inv.ID = 0
	inv.InvoiceNumber = ""
	inv.IssueDate = time.Now()
	inv.Status = "draft"

	invoiceView := views.MapInvoicePage(inv, items, "duplicate")
	a.render(w, r, "invoice_new.tmpl", map[string]any{
		"Invoice":    invoiceView,
		"Mode":       "duplicate",
		"Currencies": catalog.SupportedCurrencies,
	})
}

func (a *app) invoiceStatusHandler(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	newStatus := r.FormValue("status")

	if err := a.invRepo.UpdateInvoiceStatus(r.Context(), id, newStatus, user.ID); err != nil {
		log.Printf("status update error: %v", err)
		http.Error(w, "Could not update status", http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/invoices/%d", id), http.StatusSeeOther)
}

// =====================================================================
// Handler: PDF Generation
// =====================================================================

func (a *app) invoicePDFHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	inv, items, err := a.invRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || !canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	invoiceView := views.MapInvoicePage(inv, items, "view")

	var buf bytes.Buffer
	data := map[string]any{
		"Invoice":   invoiceView,
		"User":      auth.GetUser(r),
		"csrfField": template.HTML(""),
	}

	if err := a.templates.ExecuteTemplate(&buf, "invoice_detail.tmpl", data); err != nil {
		log.Printf("pdf template error: %v", err)
		http.Error(w, "Could not render invoice", http.StatusInternalServerError)
		return
	}

	pdfBytes, err := pdf.Generate(r.Context(), buf.String())
	if err != nil {
		log.Printf("pdf generation error: %v", err)
		http.Error(w, "Could not generate PDF", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("invoice-%s.pdf", inv.InvoiceNumber)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

// =====================================================================
// Form Parsing
// =====================================================================

type invoiceFormData struct {
	ClientID                                                                                        *int64
	CompanyName, CompanyEmail, CompanyAddress, CompanyCity, CompanyZip, CompanyState, CompanyCountry string
	ClientName, ClientEmail, ClientAddress, ClientCity, ClientZip, ClientState, ClientCountry       string
	InvoiceNumber                                                                                   string
	IssueDate                                                                                       time.Time
	DueDate                                                                                         *time.Time
	TaxRateBps                                                                                      int64
	Notes, PaymentDetails, Currency                                                                 string
	Items                                                                                           []repo.InvoiceItem
}

func parseInvoiceForm(w http.ResponseWriter, r *http.Request) (*invoiceFormData, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	if err := r.ParseForm(); err != nil {
		return nil, err
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

	if cid := r.FormValue("client_id"); cid != "" {
		if v, err := strconv.ParseInt(cid, 10, 64); err == nil {
			f.ClientID = &v
		}
	}

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

	f.TaxRateBps = parseBps(r.FormValue("tax_rate"))
	f.Currency = r.FormValue("currency")
	if f.Currency == "" {
		f.Currency = "USD"
	}

	descs := r.Form["description"]
	qtys := r.Form["quantity"]
	prices := r.Form["unit_price"]

	for i := range descs {
		if descs[i] == "" {
			continue
		}
		qty, _ := strconv.ParseFloat(qtys[i], 64)
		priceCents := parseCents(prices[i])
		f.Items = append(f.Items, repo.InvoiceItem{
			Description:    descs[i],
			Quantity:       qty,
			UnitPriceCents: priceCents,
		})
	}

	return f, nil
}

// =====================================================================
// Main
// =====================================================================

func main() {
	_ = godotenv.Load()
	initDB()
	defer db.Close()

	// Initialize PDF and Google OAuth
	pdf.Init()
	defer pdf.Shutdown()
	auth.InitGoogleOAuth()

	a := newApp()
	r := chi.NewRouter()

	r.Use(securityHeaders)
	r.Use(auth.LoadUser(a.userRepo))

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal("static fs error:", err)
	}
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	r.Get("/", a.indexHandler)
	r.Get("/tools", a.toolsHubHandler)
	r.Get("/healthz", a.healthHandler)
	r.Post("/feedback", a.feedbackHandler)

	r.Get("/register", a.registerGetHandler)
	r.Post("/register", a.registerPostHandler)
	r.Get("/login", a.loginGetHandler)
	r.Post("/login", a.loginPostHandler)
	r.Post("/logout", a.logoutHandler)

	// Google OAuth routes
	r.Get("/auth/google", auth.GoogleLoginHandler)
	r.Get("/auth/google/callback", a.googleCallbackHandler)

	// Business profile
	r.Get("/profile", a.profileGetHandler)
	r.Post("/profile", a.profilePostHandler)

	r.Route("/invoices", func(r chi.Router) {
		r.Get("/welcome", a.invoicePortalHandler)
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			if auth.GetUser(r) == nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			a.invoicesListHandler(w, r)
		})
		r.Get("/new", a.invoiceNewHandler)
		r.Post("/create", a.invoiceCreateHandler)
		r.Get("/{id}", a.invoiceDetailHandler)
		r.Get("/{id}/edit", a.invoiceEditHandler)
		r.Post("/{id}/edit", a.invoiceUpdateHandler)
		r.Get("/{id}/duplicate", a.invoiceDuplicateHandler)
		r.Post("/{id}/status", a.invoiceStatusHandler)
		r.Get("/{id}/pdf", a.invoicePDFHandler)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	csrfHandler := nosurf.New(r)
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		Secure:   os.Getenv("RAILWAY_ENVIRONMENT") != "",
		SameSite: http.SameSiteLaxMode,
	})
	csrfHandler.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("CSRF rejection: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		http.Error(w, "Forbidden — invalid or missing CSRF token", http.StatusForbidden)
	}))

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      csrfHandler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("PsiloConValley Operating on :%s [CSRF ENABLED]", port)
	log.Fatal(srv.ListenAndServe())
}
