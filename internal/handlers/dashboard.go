// internal/handlers/dashboard.go
package handlers

import (
	"log/slog"
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
		slog.Error("dashboard stats query failed", "user_id", user.ID, "err", err)
		http.Error(w, "Failed to load dashboard", http.StatusInternalServerError)
		return
	}

	// ── Recent invoices (last 10 — we filter from these) ─────────────
	recent, err := h.App.InvRepo.ListInvoices(r.Context(), 10, 0, &user.ID)
	if err != nil {
		slog.Warn("dashboard recent invoices query failed", "user_id", user.ID, "err", err)
		recent = nil
	}

	// ── Recent estimates (last 10 — we filter from these) ────────────
	estimates, err := h.App.InvRepo.ListEstimates(r.Context(), 10, 0, user.ID)
	if err != nil {
		slog.Warn("dashboard recent estimates query failed", "user_id", user.ID, "err", err)
		estimates = nil
	}

	// ── Active recurring schedules ────────────────────────────────────
	schedules, err := h.App.SchedulerRepo.ListRecurringByUserID(r.Context(), user.ID)
	if err != nil {
		slog.Warn("dashboard recurring schedules query failed", "user_id", user.ID, "err", err)
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

	// ── Onboarding checklist ─────────────────────────────────────────
	hasProfile := false
	if biz, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID); err == nil && biz != nil && biz.Name != "" {
		hasProfile = true
	}

	clientCount, _ := h.App.ClientRepo.CountByUserID(r.Context(), user.ID)
	hasClient := clientCount > 0
	// ── Expense stats (Pro/Pro Max only) ──────────────────────────────
	var monthlyExpenses int64
	var netProfitCents int64
	if canAccessExpenses(user) {
		monthlyExpenses, _ = h.App.ExpenseRepo.MonthlyTotal(r.Context(), user.ID)
	}
	netProfitCents = stats.RevenueCents - monthlyExpenses
	hasInvoice := stats.TotalCount > 0
	onboardingDone := hasProfile && hasClient && hasInvoice
	onboardingSteps := 1 // account created = always 1
	if hasProfile {
		onboardingSteps++
	}
	if hasClient {
		onboardingSteps++
	}
	if hasInvoice {
		onboardingSteps++
	}

	passkeys, _ := h.App.PasskeyRepo.GetByUserID(r.Context(), user.ID)

	// ── Quote requests from public profile ─────────────────────────
	quoteRequests, err := h.App.QuoteRequestRepo.ListByUserID(r.Context(), user.ID)
	if err != nil {
		slog.Warn("dashboard quote requests query failed", "user_id", user.ID, "err", err)
		quoteRequests = nil
	}

	newQuoteCount := 0
	for _, qr := range quoteRequests {
		if qr.Status == "new" {
			newQuoteCount++
		}
	}

	// Cap quote requests at 5 for display
	displayQuotes := quoteRequests
	if len(displayQuotes) > 5 {
		displayQuotes = displayQuotes[:5]
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
		"MonthlyExpenses": util.Money(monthlyExpenses),
		"NetProfit":       util.Money(netProfitCents),
		"NetProfitCents":  netProfitCents,
		"HasExpenses":     canAccessExpenses(user),
		"ActiveRecurring": activeRecurring,
		"IsPro":           user.Plan == "pro",
		"HasProfile":      hasProfile,
		"HasClient":       hasClient,
		"HasInvoice":      hasInvoice,
		"OnboardingDone":  onboardingDone,
		"OnboardingSteps": onboardingSteps,
		"ShowPasskeyNudge": len(passkeys) == 0,
		"QuoteRequests":    displayQuotes,
		"NewQuoteCount":    newQuoteCount,
	})
}
