package app

import (
	"html/template"
	"strings"
	"testing"

	"psiloconvalley/internal/util"
)

func TestAllTemplatesParse(t *testing.T) {
	funcs := template.FuncMap{
		"money":        util.Money,
		"formatCents":  util.FormatCentsForInput,
		"bpsToPercent": util.BpsToPercent,
		"field":        func(name, value string) string { return "" },
		"mul":          func(a, b int) int { return a * b },
		"hasPrefix":    strings.HasPrefix,
		"hasSuffix":    strings.HasSuffix,
		"seq": func(start, end int) []int {
			s := make([]int, 0, end-start+1)
			for i := start; i <= end; i++ {
				s = append(s, i)
			}
			return s
		},
	}

	tmpl, err := template.New("").Funcs(funcs).ParseGlob("../../templates/*.tmpl")
	if err != nil {
		t.Fatalf("templates/*.tmpl parse failed: %v", err)
	}

	_, err = tmpl.ParseGlob("../../templates/partials/*.tmpl")
	if err != nil {
		t.Fatalf("templates/partials/*.tmpl parse failed: %v", err)
	}

	_, err = tmpl.ParseGlob("../../templates/og/*.tmpl")
	if err != nil {
		t.Fatalf("templates/og/*.tmpl parse failed: %v", err)
	}
}


func TestInvoiceTemplatesCoverage(t *testing.T) {
	// Reconstruct the exact funcs & parse logic as app.go
	funcs := template.FuncMap{
		"money":        func(cents int64, symbol string) string { return "" },
		"formatCents":  func(cents int64) string { return "" },
		"bpsToPercent": func(bps int64) float64 { return 0.0 },
		"field":        func(name, value string) string { return "" },
		"mul":          func(a, b int) int { return a * b },
		"hasPrefix":    func(s, prefix string) bool { return false },
		"hasSuffix":    func(s, suffix string) bool { return false },
		"seq": func(start, end int) []int {
			s := make([]int, 0, end-start+1)
			for i := start; i <= end; i++ {
				s = append(s, i)
			}
			return s
		},
	}

	tmpl, err := template.New("").Funcs(funcs).ParseGlob("../../templates/*.tmpl")
	if err != nil {
		t.Fatalf("templates/*.tmpl parse failed: %v", err)
	}

	// authoritative list of registered catalog templates
	templatesToVerify := []string{"standard", "spreadsheet", "continental", "compact", "minimal", "bold"}
	
	for _, id := range templatesToVerify {
		// Map ID to filename using the canonical service name router
		// (simulate service.InvoiceTemplateName manually to decouple test path imports)
		var filename string
		switch id {
		case "minimal", "bold":
			filename = "invoice_" + id + ".tmpl"
		default:
			filename = "invoice_detail.tmpl"
		}
		
		if tmpl.Lookup(filename) == nil {
			t.Errorf("Catalog template ID %q (filename %q) was not parsed or does not exist on disk!", id, filename)
		}
	}
}
