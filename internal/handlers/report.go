// internal/handlers/report.go
package handlers

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"time"

	"psiloconvalley/internal/auth"
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
		log.Printf("[report] query error: %v", err)
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

	start, end, _ := parseDateRange(r)
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "all"
	}

	rows, err := h.App.InvRepo.ListInvoicesForReport(r.Context(), user.ID, start, end, status)
	if err != nil {
		log.Printf("[report] csv export error: %v", err)
		http.Error(w, "Failed to export", http.StatusInternalServerError)
		return
	}

	// ── Stream CSV ──────────────────────────────────────────────────
	filename := fmt.Sprintf("psiloconvalley-invoices-%s-to-%s.csv",
		start.Format("2006-01-02"), end.Format("2006-01-02"))

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
