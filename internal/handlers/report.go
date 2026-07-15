// internal/handlers/report.go
package handlers

import (
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/pdf"
	"psiloconvalley/internal/util"
)

// ── Report Page ──────────────────────────────────────────────────────────

func (h *Handlers) ReportGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	start, end, preset := parseDateRange(r)
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "all"
	}

	rows, err := h.App.InvRepo.ListInvoicesForReport(r.Context(), user.ID, start, end, status)
	if err != nil {
		slog.Error("report query failed", "user_id", user.ID, "err", err)
		http.Error(w, "Failed to load report", http.StatusInternalServerError)
		return
	}

	// ── Compute summary stats from results ──────────────────────────
	var totalCents, collectedCents, outstandingCents, overdueCents int64
	var paidCount, totalDays int
	for _, row := range rows {
		totalCents += row.TotalCents
		switch row.Status {
		case "paid":
			collectedCents += row.TotalCents
			paidCount++
			if row.DaysToPayment != nil {
				totalDays += *row.DaysToPayment
			}
		case "overdue":
			overdueCents += row.TotalCents
			outstandingCents += row.TotalCents
		case "sent":
			outstandingCents += row.TotalCents
		}
	}

	avgDays := 0
	if paidCount > 0 {
		avgDays = totalDays / paidCount
	}

	h.App.Render(w, r, "reports.tmpl", map[string]any{
		"User":         user,
		"Rows":         rows,
		"Preset":       preset,
		"StartDate":    start.Format("2006-01-02"),
		"EndDate":      end.Format("2006-01-02"),
		"Status":       status,
		"Collected":    util.Money(collectedCents),
		"Outstanding":  util.Money(outstandingCents),
		"Overdue":      util.Money(overdueCents),
		"Total":        util.Money(totalCents),
		"InvoiceCount": len(rows),
		"AvgDays":      avgDays,
	})
}

// ── CSV Export ────────────────────────────────────────────────────────────
func (h *Handlers) ReportExportCSV(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	start, end, preset := parseDateRange(r)
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "all"
	}

	rows, err := h.App.InvRepo.ListInvoicesForReport(r.Context(), user.ID, start, end, status)
	if err != nil {
		slog.Error("report csv export failed", "user_id", user.ID, "err", err)
		http.Error(w, "Failed to export", http.StatusInternalServerError)
		return
	}

	// ── Stream CSV ──────────────────────────────────────────────────
	// Filename: {business}-invoices-{status}-{preset}.csv
	bizName := "psiloconvalley"
	if biz, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID); err == nil && biz != nil && biz.Name != "" {
		bizName = slugify(biz.Name)
	}

	presetLabel := preset
	if preset == "this_month" || preset == "last_month" {
		presetLabel = preset + "-" + start.Format("Jan2006")
	} else if preset == "custom" {
		presetLabel = start.Format("Jan02") + "-to-" + end.Format("Jan02-2006")
	} else {
		presetLabel = preset + "-" + start.Format("2006")
	}
	filename := fmt.Sprintf("%s-invoices-%s-%s.csv", bizName, status, presetLabel)

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")

	cw := csv.NewWriter(w)
	defer cw.Flush()

	// Header row
	cw.Write([]string{
		"Invoice Number", "Client Name", "Client Email",
		"Issue Date", "Due Date", "Status",
		"Subtotal", "Tax", "Total",
		"Currency", "Days to Payment",
	})

	for _, row := range rows {
		dueDate := ""
		if row.DueDate != nil {
			dueDate = row.DueDate.Format("2006-01-02")
		}

		daysToPayment := ""
		if row.DaysToPayment != nil {
			daysToPayment = fmt.Sprintf("%d", *row.DaysToPayment)
		}

		cw.Write([]string{
			row.InvoiceNumber,
			row.ClientName,
			row.ClientEmail,
			row.IssueDate.Format("2006-01-02"),
			dueDate,
			row.Status,
			util.Money(row.SubtotalCents),
			util.Money(row.TaxCents),
			util.Money(row.TotalCents),
			row.Currency,
			daysToPayment,
		})
	}
}



// ── Date Range Parser ────────────────────────────────────────────────────

func parseDateRange(r *http.Request) (start, end time.Time, preset string) {
	preset = r.URL.Query().Get("preset")
	now := time.Now()

	switch preset {
	case "last_month":
		first := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
		start = first
		end = first.AddDate(0, 1, -1)
	case "q1":
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		end = time.Date(now.Year(), 3, 31, 23, 59, 59, 0, time.UTC)
	case "q2":
		start = time.Date(now.Year(), 4, 1, 0, 0, 0, 0, time.UTC)
		end = time.Date(now.Year(), 6, 30, 23, 59, 59, 0, time.UTC)
	case "q3":
		start = time.Date(now.Year(), 7, 1, 0, 0, 0, 0, time.UTC)
		end = time.Date(now.Year(), 9, 30, 23, 59, 59, 0, time.UTC)
	case "q4":
		start = time.Date(now.Year(), 10, 1, 0, 0, 0, 0, time.UTC)
		end = time.Date(now.Year(), 12, 31, 23, 59, 59, 0, time.UTC)
	case "ytd":
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		end = now
	case "custom":
		if s := r.URL.Query().Get("start"); s != "" {
			if t, err := time.Parse("2006-01-02", s); err == nil {
				start = t
			}
		}
		if e := r.URL.Query().Get("end"); e != "" {
			if t, err := time.Parse("2006-01-02", e); err == nil {
				end = t
			}
		}
		if start.IsZero() || end.IsZero() {
			start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
			end = now
			preset = "this_month"
		}
		return
	default:
		// Default: this month
		preset = "this_month"
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		end = now
	}
	return
}
// slugify converts a string to a URL/filename-safe slug.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "&", "and")
	return s
}
// ── Client Scorecard ─────────────────────────────────────────────────────

func (h *Handlers) ClientScorecardGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	cards, err := h.App.InvRepo.GetClientScorecards(r.Context(), user.ID)
	if err != nil {
		slog.Error("scorecard query failed", "user_id", user.ID, "err", err)
		http.Error(w, "Failed to load scorecards", http.StatusInternalServerError)
		return
	}

	hasPaidPlan := catalog.IsPaid(user.Plan)

	h.App.Render(w, r, "scorecard.tmpl", map[string]any{
		"User":        user,
		"Cards":       cards,
		"HasPaidPlan": hasPaidPlan,
	})
}

// ── Tax Summary ───────────────────────────────────────────────────────────

// TaxSummaryGet renders the annual tax summary page.
// Supports ?year=2024 — defaults to current year.
// Shows paid invoice revenue + expense breakdown by category.
func (h *Handlers) TaxSummaryGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	year := time.Now().Year()
	if y := r.URL.Query().Get("year"); y != "" {
		if parsed, err := fmt.Sscanf(y, "%d", &year); parsed != 1 || err != nil {
			year = time.Now().Year()
		}
	}

	data, err := h.buildTaxSummary(r, user.ID, year)
	if err != nil {
		slog.Error("tax summary failed", "user_id", user.ID, "err", err)
		http.Error(w, "Failed to load tax summary", http.StatusInternalServerError)
		return
	}

	h.App.Render(w, r, "tax_summary.tmpl", data)
}

// TaxSummaryPDFGet renders the tax summary as a downloadable PDF.
func (h *Handlers) TaxSummaryPDFGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	year := time.Now().Year()
	if y := r.URL.Query().Get("year"); y != "" {
		if parsed, err := fmt.Sscanf(y, "%d", &year); parsed != 1 || err != nil {
			year = time.Now().Year()
		}
	}

	data, err := h.buildTaxSummary(r, user.ID, year)
	if err != nil {
		slog.Error("tax summary pdf failed", "user_id", user.ID, "err", err)
		http.Error(w, "Failed to generate tax summary", http.StatusInternalServerError)
		return
	}

	// Render HTML to buffer
	var buf strings.Builder
	if err := h.App.Templates.ExecuteTemplate(&buf, "tax_summary.tmpl", data); err != nil {
		slog.Error("tax summary pdf template failed", "user_id", user.ID, "err", err)
		http.Error(w, "Failed to render tax summary", http.StatusInternalServerError)
		return
	}

	// Convert to PDF via Chromium
	pdfCtx, pdfCancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer pdfCancel()

	pdfBytes, err := pdf.Generate(pdfCtx, buf.String())
	if err != nil {
		slog.Error("tax summary pdf generate failed", "user_id", user.ID, "err", err)
		http.Error(w, "Failed to generate PDF", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("tax-summary-%d.pdf", year)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	_, _ = w.Write(pdfBytes)
}

// buildTaxSummary assembles the data map for the tax summary page and PDF.
// Extracted so both handlers share the same data logic.
func (h *Handlers) buildTaxSummary(r *http.Request, userID int64, year int) (map[string]any, error) {
	// ── Paid revenue for the year ─────────────────────────────────────
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(year, 12, 31, 23, 59, 59, 0, time.UTC)

	invoices, err := h.App.InvRepo.ListInvoicesForReport(r.Context(), userID, start, end, "paid")
	if err != nil {
		return nil, fmt.Errorf("tax summary invoices: %w", err)
	}

	var revenueCents, taxCollectedCents int64
	for _, inv := range invoices {
		revenueCents += inv.TotalCents
		taxCollectedCents += inv.TaxCents
	}

	// ── Expense breakdown for the year ───────────────────────────────
	expSummary, err := h.App.ExpenseRepo.SummaryByYear(r.Context(), userID, year)
	if err != nil {
		return nil, fmt.Errorf("tax summary expenses: %w", err)
	}

	netCents := revenueCents - expSummary.TotalCents

	// ── Year selector — current year + 3 previous ────────────────────
	currentYear := time.Now().Year()
	var years []int
	for y := currentYear; y >= currentYear-3; y-- {
		years = append(years, y)
	}

	return map[string]any{
		"Year":             year,
		"Years":            years,
		"Invoices":         invoices,
		"InvoiceCount":     len(invoices),
		"Revenue":          util.Money(revenueCents),
		"RevenueCents":     revenueCents,
		"TaxCollected":     util.Money(taxCollectedCents),
		"ExpenseTotal":     util.Money(expSummary.TotalCents),
		"ExpenseTotalCents": expSummary.TotalCents,
		"ExpenseCategories": expSummary.Categories,
		"NetIncome":        util.Money(netCents),
		"NetIncomeCents":   netCents,
	}, nil
}
