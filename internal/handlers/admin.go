// internal/handlers/admin.go
package handlers

import (
	"net/http"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/util"
)

// Admin user ID — only this user can access /admin/*
// Double lock: must match both ID and email.
const adminUserID int64 = 1
const adminEmail = "psiloconvalley408@gmail.com" // ← replace with your real email

func (h *Handlers) AdminAnalytics(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil || user.ID != adminUserID || user.Email != adminEmail {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	stats, err := h.App.InvRepo.GetAdminStats(r.Context(), h.App.DB())
	if err != nil {
		http.Error(w, "Failed to load analytics", http.StatusInternalServerError)
		return
	}

	// Calculate conversion rate
	conversionRate := 0.0
	totalResponded := stats.EstimatesAccepted + stats.EstimatesDeclined
	if totalResponded > 0 {
		conversionRate = float64(stats.EstimatesAccepted) / float64(totalResponded) * 100
	}

	// Calculate MRR (Pro users * $18.88)
	mrr := util.Money(int64(stats.ProUsers) * 1888)


	h.App.Render(w, r, "admin_analytics.tmpl", map[string]any{
		"Stats":          stats,
		"ConversionRate": conversionRate,
		"MRR":            mrr,
		"Revenue":        util.Money(stats.TotalRevenueCents),
		"Outstanding":    util.Money(stats.TotalOutstandingCents),
	})
}
