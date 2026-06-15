// internal/handlers/admin.go
package handlers

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/util"
)
func (h *Handlers) AdminAnalytics(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	adminID, err := strconv.ParseInt(os.Getenv("ADMIN_USER_ID"), 10, 64)
	adminEmail := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL")))
	if err != nil || user == nil || user.ID != adminID || strings.ToLower(strings.TrimSpace(user.Email)) != adminEmail {
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
func (h *Handlers) AdminAuditLog(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	adminID, err := strconv.ParseInt(os.Getenv("ADMIN_USER_ID"), 10, 64)
	adminEmail := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL")))
	if err != nil || user == nil || user.ID != adminID || strings.ToLower(strings.TrimSpace(user.Email)) != adminEmail {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	const pageSize = 50

	action := r.URL.Query().Get("action")

	pageStr := r.URL.Query().Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	offset := (page - 1) * pageSize

	logs, err := h.App.AuditRepo.ListRecentFiltered(r.Context(), action, pageSize, offset)
	if err != nil {
		http.Error(w, "Failed to load audit logs", http.StatusInternalServerError)
		return
	}

	total, err := h.App.AuditRepo.CountFiltered(r.Context(), action)
	if err != nil {
		http.Error(w, "Failed to count audit logs", http.StatusInternalServerError)
		return
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	h.App.Render(w, r, "admin_audit.tmpl", map[string]any{
		"Logs":       logs,
		"Page":       page,
		"TotalPages": totalPages,
		"Total":      total,
		"Action":     action,
		"PrevPage":   page - 1,
		"NextPage":   page + 1,
		"HasPrev":    page > 1,
		"HasNext":    page < totalPages,
	})
}
