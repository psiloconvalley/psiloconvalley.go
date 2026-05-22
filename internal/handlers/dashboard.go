// internal/handlers/dashboard.go
package handlers

import (
	"log"
	"net/http"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/util"
)

type DashboardData struct {
	Revenue     string
	Outstanding string
	Overdue     string
	MonthlyCount int64
	TotalCount   int64
	RecentInvoices []any
	IsPro       bool
}

func (h *Handlers) DashboardGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// ── Aggregate stats ───────────────────────────────────────────────
	stats, err := h.App.InvRepo.GetDashboardStats(r.Context(), user.ID)
	if err != nil {
		log.Printf("[dashboard] stats error: %v", err)
		http.Error(w, "Failed to load dashboard", http.StatusInternalServerError)
		return
	}

	// ── Recent invoices (last 5) ──────────────────────────────────────
	recent, err := h.App.InvRepo.ListInvoices(r.Context(), 5, 0, &user.ID)
	if err != nil {
		log.Printf("[dashboard] recent invoices error: %v", err)
		recent = nil
	}

	// ── Active recurring schedules ────────────────────────────────────
	schedules, err := h.App.SchedulerRepo.ListRecurringByUserID(r.Context(), user.ID)
	if err != nil {
		log.Printf("[dashboard] recurring schedules error: %v", err)
		schedules = nil
	}

	activeRecurring := 0
	for _, s := range schedules {
		if s.Active {
			activeRecurring++
		}
	}

	h.App.Render(w, r, "dashboard.tmpl", map[string]any{
		"User":            user,
		"Revenue":         util.Money(stats.RevenueCents),
		"Outstanding":     util.Money(stats.OutstandingCents),
		"Overdue":         util.Money(stats.OverdueCents),
		"MonthlyCount":    stats.MonthlyCount,
		"TotalCount":      stats.TotalCount,
		"RecentInvoices":  recent,
		"ActiveRecurring": activeRecurring,
		"IsPro":           user.Plan == "pro",
	})
}
