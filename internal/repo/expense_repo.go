package repo

import (
	"context"
	"database/sql"
	"fmt"
)

type ExpenseRepo struct{ db *sql.DB }

func NewExpenseRepo(db *sql.DB) *ExpenseRepo { return &ExpenseRepo{db: db} }

const expenseSelectCols = `
	SELECT e.id, e.user_id, e.amount_cents, e.currency, e.category,
		e.description, e.vendor, e.expense_date, e.receipt_url,
		e.client_id, COALESCE(c.name, ''),
		e.created_at, e.updated_at
	FROM expenses e
	LEFT JOIN clients c ON c.id = e.client_id`

func scanExpense(row interface{ Scan(...any) error }) (*Expense, error) {
	var exp Expense
	var description, vendor, receiptURL sql.NullString
	var clientID sql.NullInt64

	err := row.Scan(
		&exp.ID, &exp.UserID, &exp.AmountCents, &exp.Currency, &exp.Category,
		&description, &vendor, &exp.ExpenseDate, &receiptURL,
		&clientID, &exp.ClientName,
		&exp.CreatedAt, &exp.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if description.Valid {
		exp.Description = description.String
	}
	if vendor.Valid {
		exp.Vendor = vendor.String
	}
	if receiptURL.Valid {
		exp.ReceiptURL = receiptURL.String
	}
	if clientID.Valid {
		id := clientID.Int64
		exp.ClientID = &id
	}
	return &exp, nil
}

// List returns all expenses for a user, newest first.
func (r *ExpenseRepo) List(ctx context.Context, userID int64) ([]Expense, error) {
	rows, err := r.db.QueryContext(ctx,
		expenseSelectCols+` WHERE e.user_id = $1 ORDER BY e.expense_date DESC, e.id DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list expenses: %w", err)
	}
	defer rows.Close()

	var expenses []Expense
	for rows.Next() {
		exp, err := scanExpense(rows)
		if err != nil {
			return nil, fmt.Errorf("scan expense: %w", err)
		}
		expenses = append(expenses, *exp)
	}
	return expenses, rows.Err()
}

// GetByID returns a single expense, scoped to the user.
func (r *ExpenseRepo) GetByID(ctx context.Context, id, userID int64) (*Expense, error) {
	row := r.db.QueryRowContext(ctx,
		expenseSelectCols+` WHERE e.id = $1 AND e.user_id = $2`,
		id, userID,
	)
	return scanExpense(row)
}

// Create inserts a new expense and returns its ID.
func (r *ExpenseRepo) Create(ctx context.Context, exp *Expense) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO expenses (user_id, amount_cents, currency, category, description,
			vendor, expense_date, receipt_url, client_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, exp.UserID, exp.AmountCents, exp.Currency, exp.Category, exp.Description,
		exp.Vendor, exp.ExpenseDate, exp.ReceiptURL, exp.ClientID,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create expense: %w", err)
	}
	return id, nil
}

// Update modifies an existing expense, scoped to the user.
func (r *ExpenseRepo) Update(ctx context.Context, exp *Expense) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE expenses
		SET amount_cents = $1, currency = $2, category = $3, description = $4,
			vendor = $5, expense_date = $6, receipt_url = $7, client_id = $8,
			updated_at = NOW()
		WHERE id = $9 AND user_id = $10
	`, exp.AmountCents, exp.Currency, exp.Category, exp.Description,
		exp.Vendor, exp.ExpenseDate, exp.ReceiptURL, exp.ClientID,
		exp.ID, exp.UserID,
	)
	if err != nil {
		return fmt.Errorf("update expense: %w", err)
	}
	return nil
}

// Delete removes an expense, scoped to the user.
func (r *ExpenseRepo) Delete(ctx context.Context, id, userID int64) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM expenses WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("delete expense: %w", err)
	}
	return nil
}

// MonthlyTotal returns total expense cents for the current month.
func (r *ExpenseRepo) MonthlyTotal(ctx context.Context, userID int64) (int64, error) {
	var total sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM expenses
		WHERE user_id = $1
		AND expense_date >= date_trunc('month', CURRENT_DATE)
	`, userID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("monthly expense total: %w", err)
	}
	return total.Int64, nil
}

// TopCategories returns the top N categories by spend for the current month.
func (r *ExpenseRepo) TopCategories(ctx context.Context, userID int64, limit int) ([]CategoryTotal, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT category, COALESCE(SUM(amount_cents), 0) AS total
		FROM expenses
		WHERE user_id = $1
		AND expense_date >= date_trunc('month', CURRENT_DATE)
		GROUP BY category
		ORDER BY total DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("top categories: %w", err)
	}
	defer rows.Close()

	var cats []CategoryTotal
	for rows.Next() {
		var ct CategoryTotal
		if err := rows.Scan(&ct.Category, &ct.TotalCents); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		cats = append(cats, ct)
	}
	return cats, rows.Err()
}

// YearTotal returns total expense cents for the current calendar year.
func (r *ExpenseRepo) YearTotal(ctx context.Context, userID int64) (int64, error) {
	var total sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM expenses
		WHERE user_id = $1
		AND expense_date >= date_trunc('year', CURRENT_DATE)
	`, userID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("year expense total: %w", err)
	}
	return total.Int64, nil
}

// ExpenseYearSummary holds totals for a tax year summary.
type ExpenseYearSummary struct {
	TotalCents int64
	Categories []CategoryTotal
}

// SummaryByYear returns expense totals grouped by category for a given calendar year.
// Used by the tax summary report. Parameterized by year — works for any past year.
func (r *ExpenseRepo) SummaryByYear(ctx context.Context, userID int64, year int) (*ExpenseYearSummary, error) {
	// Total for the year
	var total sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM expenses
		WHERE user_id = $1
		AND EXTRACT(YEAR FROM expense_date) = $2
	`, userID, year).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("expense year total: %w", err)
	}

	// Category breakdown for the year
	rows, err := r.db.QueryContext(ctx, `
		SELECT category, COALESCE(SUM(amount_cents), 0) AS total
		FROM expenses
		WHERE user_id = $1
		AND EXTRACT(YEAR FROM expense_date) = $2
		GROUP BY category
		ORDER BY total DESC
	`, userID, year)
	if err != nil {
		return nil, fmt.Errorf("expense year categories: %w", err)
	}
	defer rows.Close()

	var cats []CategoryTotal
	for rows.Next() {
		var ct CategoryTotal
		if err := rows.Scan(&ct.Category, &ct.TotalCents); err != nil {
			return nil, fmt.Errorf("scan expense category: %w", err)
		}
		cats = append(cats, ct)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &ExpenseYearSummary{
		TotalCents: total.Int64,
		Categories: cats,
	}, nil
}
