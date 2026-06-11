// internal/views/invoice_template_regression_test.go
package views

import (
	"os"
	"strings"
	"testing"
)

// TestInvoiceTemplate_SyncItemInputsPresent guards against removal of
// the syncItemInputs() JavaScript function in invoice_new.tmpl.
//
// Background: invoice_new.tmpl has two responsive layouts for line items:
//   - Desktop: <tbody id="items_body_desktop"> — visible above 639px
//   - Mobile:  <div id="items_body_mobile">    — visible below 639px
//
// Both layouts share identical field names (description[], quantity[], etc.)
// CSS display:none does NOT prevent hidden inputs from submitting.
// Without syncItemInputs(), both layouts post on save — doubling line items.
//
// This was a production bug (invoice 86 had 12 items instead of 3).
// The fix disables inputs in the hidden layout before submit.
// This test ensures that fix is never silently removed.
func TestInvoiceTemplate_SyncItemInputsPresent(t *testing.T) {
	const templatePath = "../../templates/invoice_new.tmpl"

	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("could not read invoice_new.tmpl: %v", err)
	}

	tmpl := string(content)

	guards := []struct {
		name    string
		snippet string
		reason  string
	}{
		{
			name:    "syncItemInputs function defined",
			snippet: "function syncItemInputs()",
			reason:  "syncItemInputs() is the fix for dual-layout form duplication — do not remove",
		},
		{
			name:    "desktop inputs disabled on mobile",
			snippet: "el.disabled = isMobile",
			reason:  "desktop inputs must be disabled when mobile layout is active",
		},
		{
			name:    "mobile inputs disabled on desktop",
			snippet: "el.disabled = !isMobile",
			reason:  "mobile inputs must be disabled when desktop layout is active",
		},
		{
			name:    "desktop input selector present",
			snippet: "#items_body_desktop input",
			reason:  "must target desktop layout inputs to disable them on mobile",
		},
		{
			name:    "mobile input selector present",
			snippet: "#items_body_mobile input",
			reason:  "must target mobile layout inputs to disable them on desktop",
		},
		{
			name:    "resize listener wired",
			snippet: "window.addEventListener('resize', syncItemInputs)",
			reason:  "must re-sync on resize so rotating device does not re-enable wrong layout",
		},
		{
			name:    "called on page load",
			snippet: "syncItemInputs();",
			reason:  "must run on load so edit/duplicate pages start with correct layout disabled",
		},
	}

	for _, g := range guards {
		t.Run(g.name, func(t *testing.T) {
			if !strings.Contains(tmpl, g.snippet) {
				t.Errorf(
					"REGRESSION: invoice_new.tmpl is missing %q\nReason: %s",
					g.snippet,
					g.reason,
				)
			}
		})
	}
}
