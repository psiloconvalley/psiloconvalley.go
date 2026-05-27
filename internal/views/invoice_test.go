// internal/views/invoice_test.go
package views

import (
	"html/template"
	"strings"
	"testing"
)

// TestLogoURL_DataURI_NotSanitized verifies that a base64 data URI
// in LogoURL passes through html/template without being replaced
// by #ZgotmplZ.
//
// Background: Go's html/template silently replaces data: URIs in
// string-typed fields with #ZgotmplZ. LogoURL must remain typed
// as template.URL to prevent this. If anyone changes it back to
// string, this test fails immediately.
func TestLogoURL_DataURI_NotSanitized(t *testing.T) {
	// Minimal 1x1 red PNG — valid image, small enough for a test.
	const dataURI = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg=="

	tmpl := template.Must(template.New("test").Parse(
		`<img src="{{.LogoURL}}">`,
	))

	page := InvoicePage{
		LogoURL:  template.URL(dataURI),
		ShowLogo: true,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, page); err != nil {
		t.Fatalf("template execution failed: %v", err)
	}

	rendered := buf.String()

	if strings.Contains(rendered, "#ZgotmplZ") {
		t.Fatal("REGRESSION: html/template sanitized data: URI to #ZgotmplZ — LogoURL must be template.URL, not string")
	}

	if !strings.Contains(rendered, "data:image/png;base64,") {
		t.Fatalf("expected data URI prefix in rendered output, got: %s", rendered)
	}
}

// TestLogoURL_HTTPS_NotSanitized verifies that a normal HTTPS URL
// also passes through cleanly. This is a baseline sanity check.
func TestLogoURL_HTTPS_NotSanitized(t *testing.T) {
	const url = "https://example.supabase.co/storage/v1/object/public/logos/logo-user-1.png"

	tmpl := template.Must(template.New("test").Parse(
		`<img src="{{.LogoURL}}">`,
	))

	page := InvoicePage{
		LogoURL:  template.URL(url),
		ShowLogo: true,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, page); err != nil {
		t.Fatalf("template execution failed: %v", err)
	}

	rendered := buf.String()

	if strings.Contains(rendered, "#ZgotmplZ") {
		t.Fatal("REGRESSION: html/template sanitized HTTPS URL to #ZgotmplZ")
	}

	if !strings.Contains(rendered, url) {
		t.Fatalf("expected URL in rendered output, got: %s", rendered)
	}
}

// TestLogoURL_Empty_NoImage verifies that an empty LogoURL does not
// produce broken markup.
func TestLogoURL_Empty_NoImage(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse(
		`{{if .LogoURL}}<img src="{{.LogoURL}}">{{end}}`,
	))

	page := InvoicePage{
		LogoURL:  "",
		ShowLogo: true,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, page); err != nil {
		t.Fatalf("template execution failed: %v", err)
	}

	rendered := buf.String()

	if strings.Contains(rendered, "<img") {
		t.Fatalf("expected no <img> tag for empty LogoURL, got: %s", rendered)
	}
}
