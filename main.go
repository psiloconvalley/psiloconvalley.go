package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	// Adjust this import path to match your module name in go.mod
	"psiloconvalley/internal/repo"
)

var (
	templates  *template.Template
	invRepo    *repo.InvoiceRepo
	clientRepo *repo.ClientRepo
	bizRepo    *repo.BusinessRepo
)

// Template helper: if you decide to keep amounts in cents, this is handy
func mulcents(cents int64) float64 {
	return float64(cents) / 100.0
}

func initTemplates() {
	funcs := template.FuncMap{
		"mulcents": mulcents,
		// add more helpers as needed (formatDate, money, etc.)
	}

	var err error
	// Templates live in ./templates/*.tmpl (adjust pattern if yours differ)
	templates, err = template.New("").Funcs(funcs).ParseGlob("templates/*.tmpl")
	if err != nil {
		log.Fatal("error parsing templates:", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if err := db.Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintln(w, "db unavailable")
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

// Home / landing page
func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if err := templates.ExecuteTemplate(w, "home.tmpl", nil); err != nil {
		log.Println("home template error:", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
	}
}

// GET /invoices/new -> show form
func invoiceNewHandler(w http.ResponseWriter, r *http.Request) {
	// In a real app, load clients/business profile(s) to populate selects
	data := map[string]any{
		// "Clients": clientRepo.List(...),
		// "Business": bizRepo.GetDefault(...),
	}
	if err := templates.ExecuteTemplate(w, "invoice_new.tmpl", data); err != nil {
		log.Println("invoice_new template error:", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
	}
}

// POST /invoices -> create invoice
func invoiceCreateHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	// Minimal extraction (expand with validation as needed)
	bizID, _ := strconv.ParseInt(r.FormValue("business_profile_id"), 10, 64)
	var clientIDPtr *int64
	if cid := r.FormValue("client_id"); cid != "" {
		if v, err := strconv.ParseInt(cid, 10, 64); err == nil {
			clientIDPtr = &v
		}
	}

	issueDate, _ := time.Parse("2006-01-02", r.FormValue("issue_date"))
	var dueDatePtr *time.Time
	if dd := r.FormValue("due_date"); dd != "" {
		if t, err := time.Parse("2006-01-02", dd); err == nil {
			dueDatePtr = &t
		}
	}

	taxRate, _ := strconv.ParseFloat(r.FormValue("tax_rate"), 64)
	discount, _ := strconv.ParseFloat(r.FormValue("discount_amount"), 64)

	inv := &repo.Invoice{
		BusinessProfileID: bizID,
		ClientID:          clientIDPtr,
		InvoiceNumber:     r.FormValue("invoice_number"),
		IssueDate:         issueDate,
		DueDate:           dueDatePtr,
		Status:            "draft",
		TaxRate:           taxRate,
		DiscountAmount:    discount,
		Notes:             r.FormValue("notes"),
		PaymentDetails:    r.FormValue("payment_details"),
	}

	// Collect line items (simple approach; improve with JS add/remove later)
	items := []repo.InvoiceItem{}
	descs := r.Form["description"]
	qtys := r.Form["quantity"]
	prices := r.Form["unit_price"]
	for i := range descs {
		if descs[i] == "" {
			continue
		}
		qty, _ := strconv.ParseFloat(qtys[i], 64)
		price, _ := strconv.ParseFloat(prices[i], 64)
		items = append(items, repo.InvoiceItem{
			Description: descs[i],
			Quantity:    qty,
			UnitPrice:   price,
		})
	}

	id, err := invRepo.CreateInvoice(inv, items)
	if err != nil {
		log.Println("create invoice error:", err)
		http.Error(w, "could not create invoice", http.StatusInternalServerError)
		return
	}

	// Redirect to invoice detail
	http.Redirect(w, r, fmt.Sprintf("/invoices/%d", id), http.StatusSeeOther)
}

// GET /invoices/{id} -> show invoice
func invoiceDetailHandler(w http.ResponseWriter, r *http.Request) {
	// naive path param parsing (use gorilla/mux chi later for nicer routing)
	idStr := r.URL.Path[len("/invoices/"):]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	inv, items, err := invRepo.GetInvoiceWithItems(id)
	if err != nil {
		log.Println("get invoice error:", err)
		http.NotFound(w, r)
		return
	}
	data := map[string]any{
		"Invoice": inv,
		"Items":   items,
		// compute totals in template helper or here
	}
	if err := templates.ExecuteTemplate(w, "invoice_detail.tmpl", data); err != nil {
		log.Println("invoice_detail template error:", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
	}
}

// GET /invoices -> list invoices
func invoicesListHandler(w http.ResponseWriter, r *http.Request) {
	list, err := invRepo.ListInvoices(50, 0) // simple pagination
	if err != nil {
		log.Println("list invoices error:", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	if err := templates.ExecuteTemplate(w, "invoices_list.tmpl", map[string]any{"Invoices": list}); err != nil {
		log.Println("invoices_list template error:", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
	}
}

func main() {
	// DB connection (Railway provides DATABASE_URL)
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	var err error
	db, err = sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatal("failed to open db:", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("failed to ping db:", err)
	}

	// Initialize repos
	invRepo = repo.NewInvoiceRepo(db)
	clientRepo = repo.NewClientRepo(db)
	bizRepo = repo.NewBusinessRepo(db)

	// Templates
	initTemplates()

	// Router
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/", homeHandler)
	mux.HandleFunc("/invoices", invoicesListHandler)         // GET list
	mux.HandleFunc("/invoices/new", invoiceNewHandler)       // GET form
	mux.HandleFunc("/invoices/", invoiceDetailHandler)       // GET detail (simple path)
	mux.HandleFunc("/invoices/create", invoiceCreateHandler) // POST

	// Port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
