package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"

	"psiloconvalley/internal/repo"
)

var (
	templates  *template.Template
	invRepo    *repo.InvoiceRepo
	clientRepo *repo.ClientRepo
	bizRepo    *repo.BusinessRepo
)

func money(v float64) string {
	return fmt.Sprintf("$%.2f", v)
}

func mulcents(cents int64) float64 {
	return float64(cents) / 100.0
}

func initTemplates() {
	funcs := template.FuncMap{
		"money":    money,
		"mulcents": mulcents,
	}

	var err error

	templates, err = template.New("").
		Funcs(funcs).
		ParseGlob("templates/*.tmpl")

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

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := templates.ExecuteTemplate(w, "home.tmpl", nil)
	if err != nil {
		log.Println("home template error:", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
	}
}

func invoiceNewHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := templates.ExecuteTemplate(w, "invoice_new.tmpl", nil)
	if err != nil {
		log.Println("invoice_new template error:", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
	}
}

func invoiceCreateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/invoices/new", http.StatusSeeOther)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	var businessProfileID *int64

	if bid := r.FormValue("business_profile_id"); bid != "" {
		if v, err := strconv.ParseInt(bid, 10, 64); err == nil {
			businessProfileID = &v
		}
	}

	var clientID *int64

	if cid := r.FormValue("client_id"); cid != "" {
		if v, err := strconv.ParseInt(cid, 10, 64); err == nil {
			clientID = &v
		}
	}

	issueDate := time.Now()

	if val := r.FormValue("issue_date"); val != "" {
		if t, err := time.Parse("2006-01-02", val); err == nil {
			issueDate = t
		}
	}

	var dueDate *time.Time

	if val := r.FormValue("due_date"); val != "" {
		if t, err := time.Parse("2006-01-02", val); err == nil {
			dueDate = &t
		}
	}

	taxRate, _ := strconv.ParseFloat(r.FormValue("tax_rate"), 64)

	inv := &repo.Invoice{
		BusinessProfileID: businessProfileID,
		ClientID:          clientID,
		ClientName:        r.FormValue("client_name"),
		InvoiceNumber:     r.FormValue("invoice_number"),
		IssueDate:         issueDate,
		DueDate:           dueDate,
		TaxRate:           taxRate,
		Notes:             r.FormValue("notes"),
		PaymentDetails:    r.FormValue("payment_details"),
		Status:            "draft",
	}

	var items []repo.InvoiceItem

	descs := r.Form["description"]
	qtys := r.Form["quantity"]
	prices := r.Form["unit_price"]

	for i := range descs {
		if descs[i] == "" {
			continue
		}

		qty := 1.0
		price := 0.0

		if i < len(qtys) {
			qty, _ = strconv.ParseFloat(qtys[i], 64)
		}

		if i < len(prices) {
			price, _ = strconv.ParseFloat(prices[i], 64)
		}

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

	http.Redirect(w, r, fmt.Sprintf("/invoices/%d", id), http.StatusSeeOther)
}

func invoiceDetailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
	}

	err = templates.ExecuteTemplate(w, "invoice_detail.tmpl", data)
	if err != nil {
		log.Println("invoice_detail template error:", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
	}
}

func invoicesListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	list, err := invRepo.ListInvoices(50, 0)
	if err != nil {
		log.Println("list invoices error:", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Invoices": list,
	}

	err = templates.ExecuteTemplate(w, "invoices_list.tmpl", data)
	if err != nil {
		log.Println("invoices_list template error:", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
	}
}
func invoiceDuplicateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/invoices", http.StatusSeeOther)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	idStr := r.FormValue("id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	oldInvoice, oldItems, err := invRepo.GetInvoiceWithItems(id)
	if err != nil {
		log.Println("duplicate invoice get error:", err)
		http.Error(w, "could not duplicate invoice", http.StatusInternalServerError)
		return
	}

	newInvoice := *oldInvoice

	newInvoice.ID = 0
	newInvoice.InvoiceNumber = ""
	newInvoice.Status = "draft"
	newInvoice.IssueDate = time.Now()
	newInvoice.DueDate = nil
	newInvoice.CreatedAt = time.Time{}

	newItems := make([]repo.InvoiceItem, 0, len(oldItems))

	for _, item := range oldItems {
		newItems = append(newItems, repo.InvoiceItem{
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
		})
	}

	newID, err := invRepo.CreateInvoice(&newInvoice, newItems)
	if err != nil {
		log.Println("duplicate invoice create error:", err)
		http.Error(w, "could not duplicate invoice", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/invoices/%d", newID), http.StatusSeeOther)
}



func main() {
	_ = godotenv.Load()

	initDB()
	defer db.Close()

	initTemplates()

	invRepo = repo.NewInvoiceRepo(db)
	clientRepo = repo.NewClientRepo(db)
	bizRepo = repo.NewBusinessRepo(db)

	mux := http.NewServeMux()
	
	mux.Handle("/static/", http.StripPrefix("/static/", 
	http.FileServer(http.Dir("static"))))
	
	mux.HandleFunc("/", homeHandler)
	mux.HandleFunc("/healthz", healthHandler)

	mux.HandleFunc("/invoices", invoicesListHandler)
	mux.HandleFunc("/invoices/new", invoiceNewHandler)
	mux.HandleFunc("/invoices/create", invoiceCreateHandler)
	mux.HandleFunc("/invoices/duplicate", invoiceDuplicateHandler)
	mux.HandleFunc("/invoices/", invoiceDetailHandler)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on :%s", port)

	log.Fatal(http.ListenAndServe(":"+port, mux))
}
