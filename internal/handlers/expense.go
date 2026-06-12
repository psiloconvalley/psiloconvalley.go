package handlers

import (
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/receipt"
	"psiloconvalley/internal/repo"
	"psiloconvalley/internal/util"
)

// canAccessExpenses returns true if the user's plan includes expense tracking.
func canAccessExpenses(user *repo.User) bool {
	return user.Plan == "growth" || user.Plan == "pro"
}

func (h *Handlers) ExpensesList(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !canAccessExpenses(user) {
		http.Redirect(w, r, "/pricing?reason=expenses", http.StatusSeeOther)
		return
	}

	expenses, err := h.App.ExpenseRepo.List(r.Context(), user.ID)
	if err != nil {
		log.Printf("[expenses] list error: %v", err)
	}

	monthTotal, _ := h.App.ExpenseRepo.MonthlyTotal(r.Context(), user.ID)
	yearTotal, _ := h.App.ExpenseRepo.YearTotal(r.Context(), user.ID)

	h.App.Render(w, r, "expenses_list.tmpl", map[string]any{
		"Expenses":   expenses,
		"MonthTotal": util.Money(monthTotal),
		"YearTotal":  util.Money(yearTotal),
		"Deleted":    r.URL.Query().Get("deleted") == "true",
		"Saved":      r.URL.Query().Get("saved") == "true",
	})
}

func (h *Handlers) ExpenseNewGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !canAccessExpenses(user) {
		http.Redirect(w, r, "/pricing?reason=expenses", http.StatusSeeOther)
		return
	}

	clients, _ := h.App.ClientRepo.ListByUserID(r.Context(), user.ID)

	// Default currency from business profile
	currency := "USD"
	if bp, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID); err == nil && bp != nil {
		if bp.Currency != "" {
			currency = bp.Currency
		}
	}

	h.App.Render(w, r, "expense_new.tmpl", map[string]any{
		"Mode":       "create",
		"Categories": catalog.ExpenseCategories,
		"Clients":    clients,
		"Currency":   currency,
		"Today":      time.Now().Format("2006-01-02"),
	})
}

func (h *Handlers) ExpenseCreatePost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !canAccessExpenses(user) {
		http.Redirect(w, r, "/pricing?reason=expenses", http.StatusSeeOther)
		return
	}

	if err := r.ParseMultipartForm(6 << 20); err != nil {
		log.Printf("[expenses] parse form error: %v", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	exp := parseExpenseForm(r, user.ID)

	if exp.Category == "" || exp.AmountCents <= 0 {
		clients, _ := h.App.ClientRepo.ListByUserID(r.Context(), user.ID)
		h.App.Render(w, r, "expense_new.tmpl", map[string]any{
			"Mode":       "create",
			"Error":      "Category and amount are required.",
			"Expense":    exp,
			"Categories": catalog.ExpenseCategories,
			"Clients":    clients,
			"Currency":   exp.Currency,
			"Today":      time.Now().Format("2006-01-02"),
			"SelectedClientID": selectedClientID(exp),
		})
		return
	}

	id, err := h.App.ExpenseRepo.Create(r.Context(), exp)
	if err != nil {
		log.Printf("[expenses] create error: %v", err)
		http.Error(w, "Could not save expense", http.StatusInternalServerError)
		return
	}

	// Handle receipt upload
	h.handleReceiptUpload(r, user.ID, id, exp)

	if exp.ReceiptURL != "" {
		if err := h.App.ExpenseRepo.Update(r.Context(), exp); err != nil {
			log.Printf("[expenses] update receipt URL error: %v", err)
		}
	}

	http.Redirect(w, r, "/expenses?saved=true", http.StatusSeeOther)
}

func (h *Handlers) ExpenseEditGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !canAccessExpenses(user) {
		http.Redirect(w, r, "/pricing?reason=expenses", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	exp, err := h.App.ExpenseRepo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	clients, _ := h.App.ClientRepo.ListByUserID(r.Context(), user.ID)

	h.App.Render(w, r, "expense_new.tmpl", map[string]any{
		"Mode":       "edit",
		"Expense":    exp,
		"Categories": catalog.ExpenseCategories,
		"Clients":    clients,
		"Currency":   exp.Currency,
		"SelectedClientID": selectedClientID(exp),
	})
}

func (h *Handlers) ExpenseEditPost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !canAccessExpenses(user) {
		http.Redirect(w, r, "/pricing?reason=expenses", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Verify ownership
	existing, err := h.App.ExpenseRepo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseMultipartForm(6 << 20); err != nil {
		log.Printf("[expenses] parse form error: %v", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	exp := parseExpenseForm(r, user.ID)
	exp.ID = id

	// Preserve existing receipt if no new upload
	if exp.ReceiptURL == "" {
		exp.ReceiptURL = existing.ReceiptURL
	}

	if exp.Category == "" || exp.AmountCents <= 0 {
		clients, _ := h.App.ClientRepo.ListByUserID(r.Context(), user.ID)
		h.App.Render(w, r, "expense_new.tmpl", map[string]any{
			"Mode":       "edit",
			"Error":      "Category and amount are required.",
			"Expense":    exp,
			"Categories": catalog.ExpenseCategories,
			"Clients":    clients,
			"Currency":   exp.Currency,
			"SelectedClientID": selectedClientID(exp),
		})
		return
	}

	// Handle receipt upload
	h.handleReceiptUpload(r, user.ID, id, exp)

	if err := h.App.ExpenseRepo.Update(r.Context(), exp); err != nil {
		log.Printf("[expenses] update error: %v", err)
		http.Error(w, "Could not update expense", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/expenses?saved=true", http.StatusSeeOther)
}

func (h *Handlers) ExpenseDeletePost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !canAccessExpenses(user) {
		http.Redirect(w, r, "/pricing?reason=expenses", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Delete receipt file
	_ = h.App.ReceiptStore.Delete(user.ID, id)

	if err := h.App.ExpenseRepo.Delete(r.Context(), id, user.ID); err != nil {
		log.Printf("[expenses] delete error: %v", err)
	}

	http.Redirect(w, r, "/expenses?deleted=true", http.StatusSeeOther)
}

// =====================================================================
// Helpers
// =====================================================================

func parseExpenseForm(r *http.Request, userID int64) *repo.Expense {
	amountStr := strings.TrimSpace(r.FormValue("amount"))
	amountFloat, _ := strconv.ParseFloat(amountStr, 64)
	amountCents := int64(math.Round(amountFloat * 100))

	category := strings.TrimSpace(r.FormValue("category"))
	if !catalog.ValidExpenseCategory(category) {
		category = ""
	}

	currency := strings.TrimSpace(r.FormValue("currency"))
	if currency == "" {
		currency = "USD"
	}

	expenseDate, _ := time.Parse("2006-01-02", r.FormValue("expense_date"))
	if expenseDate.IsZero() {
		expenseDate = time.Now()
	}

	var clientID *int64
	if cidStr := r.FormValue("client_id"); cidStr != "" {
		if cid, err := strconv.ParseInt(cidStr, 10, 64); err == nil && cid > 0 {
			clientID = &cid
		}
	}

	return &repo.Expense{
		UserID:      userID,
		AmountCents: amountCents,
		Currency:    currency,
		Category:    category,
		Description: strings.TrimSpace(r.FormValue("description")),
		Vendor:      strings.TrimSpace(r.FormValue("vendor")),
		ExpenseDate: expenseDate,
		ClientID:    clientID,
	}
}

func (h *Handlers) handleReceiptUpload(r *http.Request, userID, expenseID int64, exp *repo.Expense) {
	file, header, err := r.FormFile("receipt")
	if err != nil {
		return // no file uploaded — not an error
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		log.Printf("[expenses] receipt read error: %v", err)
		return
	}

	ext, err := receipt.ValidateFile(data, header.Filename)
	if err != nil {
		log.Printf("[expenses] receipt validation error: %v", err)
		return
	}

	url, err := h.App.ReceiptStore.Save(userID, expenseID, data, ext)
	if err != nil {
		log.Printf("[expenses] receipt save error: %v", err)
		return
	}

	exp.ID = expenseID
	exp.ReceiptURL = url
}
func selectedClientID(exp *repo.Expense) int64 {
	if exp != nil && exp.ClientID != nil {
		return *exp.ClientID
	}
	return 0
}
