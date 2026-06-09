// internal/handlers/dashboard.go
package handlers

import (
	"log"
	"net/http"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/repo"
	"psiloconvalley/internal/util"
)

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

	// ── Recent invoices (last 10 — we filter from these) ─────────────
	recent, err := h.App.InvRepo.ListInvoices(r.Context(), 10, 0, &user.ID)
	if err != nil {
		log.Printf("[dashboard] recent invoices error: %v", err)
		recent = nil
	}

	// ── Recent estimates (last 10 — we filter from these) ────────────
	estimates, err := h.App.InvRepo.ListEstimates(r.Context(), 10, 0, user.ID)
	if err != nil {
		log.Printf("[dashboard] recent estimates error: %v", err)
		estimates = nil
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

	// ── Build "Needs Attention" list ─────────────────────────────────
	var needsAttention []repo.Invoice

	// Overdue and sent invoices need attention
	for _, inv := range recent {
		if inv.Status == "overdue" || inv.Status == "sent" {
			needsAttention = append(needsAttention, inv)
		}
	}

	// Pending estimates (sent but not accepted/declined) need attention
	for _, est := range estimates {
		if est.Status == "sent" || est.Status == "draft" {
			needsAttention = append(needsAttention, est)
		}
	}

	// Cap recent tables at 5 for display
	displayInvoices := recent
	if len(displayInvoices) > 5 {
		displayInvoices = displayInvoices[:5]
	}

	displayEstimates := estimates
	if len(displayEstimates) > 5 {
		displayEstimates = displayEstimates[:5]
	}

	h.App.Render(w, r, "dashboard.tmpl", map[string]any{
		"User":            user,
		"Revenue":         util.Money(stats.RevenueCents),
		"Outstanding":     util.Money(stats.OutstandingCents),
		"Overdue":         util.Money(stats.OverdueCents),
		"MonthlyCount":    stats.MonthlyCount,
		"TotalCount":      stats.TotalCount,
		"RecentInvoices":  displayInvoices,
		"RecentEstimates": displayEstimates,
		"NeedsAttention":  needsAttention,
		"ActiveRecurring": activeRecurring,
		"IsPro":           user.Plan == "pro",
	})
}
