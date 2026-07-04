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
