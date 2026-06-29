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
	"psiloconvalley/internal/catalog"
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

	// Map header names to column indices (case-insensitive).
	// Supports common variations so users don't have to match exactly.
	headerAliases := map[string]string{
		"client name":  "name",
		"client":       "name",
		"nombre":       "name",
		"correo":       "email",
		"email address":"email",
		"telefono":     "phone",
		"teléfono":     "phone",
		"telephone":    "phone",
		"street":       "address",
		"direccion":    "address",
		"dirección":    "address",
		"street address": "address",
		"ciudad":       "city",
		"estado":       "state",
		"province":     "state",
		"region":       "state",
		"postal code":  "zip",
		"zip code":     "zip",
		"codigo postal":"zip",
		"código postal":"zip",
		"pais":         "country",
		"país":         "country",
		"notas":        "notes",
	}

	colMap := make(map[string]int)
	for i, col := range header {
		key := strings.ToLower(strings.TrimSpace(col))
		// Check if this header is an alias for a standard column
		if canonical, ok := headerAliases[key]; ok {
			colMap[canonical] = i
		} else {
			colMap[key] = i
		}
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
			catalog.NormalizeState(getCol(record, colMap, "state")),
			getCol(record, colMap, "zip"),
			catalog.NormalizeCountry(getCol(record, colMap, "country")),
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

// ClientImportTemplate serves a downloadable CSV template with headers + example row.
// Route: GET /clients/import/template.csv
func (h *Handlers) ClientImportTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="client_import_template.csv"`)

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header row — shared with export
	_ = writer.Write(clientCSVHeaders)

	// Example row — shows the user what to fill in
	_ = writer.Write([]string{
		"Example Client", "client@email.com", "555-123-4567",
		"123 Main St", "San Jose", "CA", "95112", "United States",
		"Monthly pool service",
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
