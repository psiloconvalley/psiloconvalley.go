// internal/handlers/client_export.go
//
// CSV export for client records.
// Streams the response — no buffering the entire file in memory.
// Authenticated — only the logged-in user's clients are exported.
package handlers

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"psiloconvalley/internal/auth"
)

// clientCSVHeaders defines the column order for export AND import.
// Centralised here so both operations share the same contract.
var clientCSVHeaders = []string{
	"Name", "Email", "Phone", "Address", "City",
	"State", "Zip", "Country", "Notes",
}

// ClientExportCSV streams all of the user's clients as a CSV download.
// Route: GET /clients/export.csv
func (h *Handlers) ClientExportCSV(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	clients, err := h.App.ClientRepo.ListByUserID(r.Context(), user.ID)
	if err != nil {
		slog.Error("client export failed", "user_id", user.ID, "err", err)
		http.Error(w, "Could not export clients", http.StatusInternalServerError)
		return
	}

	// Set headers for CSV download
	filename := fmt.Sprintf("clients_%s.csv", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header row
	if err := writer.Write(clientCSVHeaders); err != nil {
		slog.Error("client export csv header write failed", "err", err)
		return
	}

	// Write data rows
	for _, c := range clients {
		row := []string{
			c.Name, c.Email, c.Phone, c.Address, c.City,
			c.State, c.Zip, c.Country, c.Notes,
		}
		if err := writer.Write(row); err != nil {
			slog.Error("client export csv row write failed", "client_id", c.ID, "err", err)
			return
		}
	}

	slog.Info("client export completed", "user_id", user.ID, "count", len(clients))
}
