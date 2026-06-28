// internal/handlers/client_import.go
//
// CSV import for client records.
// Uses FindOrCreate to prevent duplicates — matches by name.
// Shares clientCSVHeaders with client_export.go for format consistency.
package handlers

import (
	"encoding/csv"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"psiloconvalley/internal/auth"
)

const (
	maxImportFileSize = 1 << 20 // 1MB
	maxImportRows     = 500
)

// ImportResult holds the outcome of a CSV import for display.
type ImportResult struct {
	Imported int
	Skipped  int
	Errors   []string
}

// ClientImportGet renders the CSV upload form.
// Route: GET /clients/import
func (h *Handlers) ClientImportGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	h.App.Render(w, r, "client_import.tmpl", nil)
}

// ClientImportPost parses an uploaded CSV and creates clients.
// Route: POST /clients/import
func (h *Handlers) ClientImportPost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Limit upload size
	r.Body = http.MaxBytesReader(w, r.Body, maxImportFileSize)
	if err := r.ParseMultipartForm(maxImportFileSize); err != nil {
		h.App.Render(w, r, "client_import.tmpl", map[string]any{
			"Error": "File too large. Maximum 1MB.",
		})
		return
	}

	// Get business profile — required for client ownership
	profile, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID)
	if err != nil || profile == nil {
		h.App.Render(w, r, "client_import.tmpl", map[string]any{
			"Error": "Please set up your business profile first.",
		})
		return
	}

	// Read uploaded file
	file, _, err := r.FormFile("csv_file")
	if err != nil {
		h.App.Render(w, r, "client_import.tmpl", map[string]any{
			"Error": "Please select a CSV file.",
		})
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1 // allow variable columns

	// Read header row
	header, err := reader.Read()
	if err != nil {
		h.App.Render(w, r, "client_import.tmpl", map[string]any{
			"Error": "Could not read CSV header row.",
		})
		return
	}

	// Map header names to column indices (case-insensitive)
	colMap := make(map[string]int)
	for i, col := range header {
		colMap[strings.ToLower(strings.TrimSpace(col))] = i
	}

	// Validate required "name" column exists
	if _, ok := colMap["name"]; !ok {
		h.App.Render(w, r, "client_import.tmpl", map[string]any{
			"Error": "CSV must have a 'Name' column.",
		})
		return
	}

	// Process rows
	result := ImportResult{}
	rowNum := 1 // 1-indexed after header

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, "Row "+itoa(rowNum)+": could not parse")
			rowNum++
			continue
		}

		if rowNum > maxImportRows {
			result.Errors = append(result.Errors, "Maximum "+itoa(maxImportRows)+" rows exceeded. Remaining rows skipped.")
			break
		}

		name := getCol(record, colMap, "name")
		if name == "" {
			result.Skipped++
			rowNum++
			continue
		}

		_, err = h.App.ClientRepo.FindOrCreate(
			r.Context(),
			profile.ID,
			name,
			getCol(record, colMap, "email"),
			getCol(record, colMap, "address"),
			getCol(record, colMap, "city"),
			getCol(record, colMap, "state"),
			getCol(record, colMap, "zip"),
			getCol(record, colMap, "country"),
		)
		if err != nil {
			result.Errors = append(result.Errors, "Row "+itoa(rowNum)+" ("+name+"): "+err.Error())
			result.Skipped++
		} else {
			result.Imported++
		}

		rowNum++
	}

	slog.Info("client import completed",
		"user_id", user.ID,
		"imported", result.Imported,
		"skipped", result.Skipped,
		"errors", len(result.Errors),
	)

	h.App.Render(w, r, "client_import.tmpl", map[string]any{
		"Result": result,
	})
}

// ── Helpers ──────────────────────────────────────────────────────────

// getCol safely retrieves a column value by header name.
// Returns empty string if column doesn't exist or index is out of range.
func getCol(record []string, colMap map[string]int, col string) string {
	idx, ok := colMap[strings.ToLower(col)]
	if !ok || idx >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[idx])
}

// itoa avoids importing strconv for a simple int-to-string in error messages.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
