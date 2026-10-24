package catalog

import "testing"

func TestNormalizeCurrency(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"USD", "USD"},
		{"usd", "USD"},
		{"  CAD  ", "CAD"},
		{"eur", "EUR"},
		{"MXN", "MXN"},
		{"GBP", "GBP"},
		{"", "USD"},            // empty → default
		{"XYZ", "USD"},         // unknown → default
		{"US Dollar", "USD"},   // name instead of code → default (not matched)
		{"us", "USD"},          // partial → default
	}
	for _, c := range cases {
		if got := NormalizeCurrency(c.in); got != c.want {
			t.Errorf("NormalizeCurrency(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCurrencySymbol(t *testing.T) {
	cases := []struct {
		code, want string
	}{
		{"USD", "$"},
		{"CAD", "CA$"},
		{"GBP", "£"},
		{"EUR", "€"},
		{"MXN", "MX$"},
		{"usd", "$"},         // normalized
		{"XYZ", "$"},         // unknown → default USD symbol
		{"", "$"},            // empty → default
	}
	for _, c := range cases {
		if got := CurrencySymbol(c.code); got != c.want {
			t.Errorf("CurrencySymbol(%q) = %q, want %q", c.code, got, c.want)
		}
	}
}

func TestNormalizeState(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"CA", "California"},
		{"ca", "California"},
		{"Ca", "California"},
		{"NY", "New York"},
		{"california", "California"},
		{"CALIFORNIA", "California"},
		{"New York", "New York"},
		{"new york", "New York"},
		{"TX", "Texas"},
		{"", ""},                           // empty → unchanged
		{"  CA  ", "California"},           // trim then normalize
		{"Ontario", "Ontario"},             // non-US → passthrough
		{"ZZ", "ZZ"},                       // unknown abbrev → passthrough
		{"Not A State", "Not A State"},     // unknown name → passthrough
	}
	for _, c := range cases {
		if got := NormalizeState(c.in); got != c.want {
			t.Errorf("NormalizeState(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeCountry(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"US", "United States"},
		{"USA", "United States"},
		{"usa", "United States"},
		{"U.S.", "United States"},
		{"U.S.A.", "United States"},
		{"united states", "United States"},
		{"UNITED STATES OF AMERICA", "United States"},
		{"MX", "Mexico"},
		{"MEXICO", "Mexico"},
		{"México", "Mexico"},
		{"MÉXICO", "Mexico"},
		{"CA", "Canada"},
		{"canada", "Canada"},
		{"GB", "United Kingdom"},
		{"UK", "United Kingdom"},
		{"", ""},                       // empty → unchanged
		{"  US  ", "United States"},    // trim then normalize
		{"France", "France"},           // unknown → passthrough
		{"Germany", "Germany"},         // unknown → passthrough
	}
	for _, c := range cases {
		if got := NormalizeCountry(c.in); got != c.want {
			t.Errorf("NormalizeCountry(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatPhone(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		// Standard US — 10 digits
		{"5551234567", "(555) 123-4567"},
		{"555-123-4567", "(555) 123-4567"},
		{"555.123.4567", "(555) 123-4567"},
		{"(555) 123-4567", "(555) 123-4567"},
		{"555 123 4567", "(555) 123-4567"},

		// US with country code — 11 digits starting with 1
		{"15551234567", "+1 (555) 123-4567"},
		{"1-555-123-4567", "+1 (555) 123-4567"},

		// With + prefix
		{"+15551234567", "+1 (555) 123-4567"},
		{"+1 555 123 4567", "+1 (555) 123-4567"},
		{"+1 (555) 123-4567", "+1 (555) 123-4567"},

		// 7-digit local
		{"1234567", "123-4567"},
		{"123-4567", "123-4567"},

		// Edge cases
		{"", ""},
		{"   ", ""},
		{"abc", "abc"},                           // no digits → passthrough
		{"123", "123"},                           // too few → passthrough
		{"12345678901234", "12345678901234"},     // too many → passthrough
		{"+447911123456", "+447911123456"},       // UK, non-US → passthrough
	}
	for _, c := range cases {
		if got := FormatPhone(c.in); got != c.want {
			t.Errorf("FormatPhone(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDefaultIfBlank(t *testing.T) {
	cases := []struct {
		value, fallback, want string
	}{
		{"hello", "default", "hello"},
		{"", "default", "default"},
		{"   ", "default", "default"},         // whitespace treated as blank
		{"\t\n", "default", "default"},        // tabs/newlines treated as blank
		{"  real  ", "default", "real"},       // Note: this currently FAILS because func does not trim-return
	}
	for _, c := range cases {
		got := DefaultIfBlank(c.value, c.fallback)
		if got != c.want {
			t.Errorf("DefaultIfBlank(%q, %q) = %q, want %q", c.value, c.fallback, got, c.want)
		}
	}
}

func TestSupportedCurrenciesIntegrity(t *testing.T) {
	// Sanity: no duplicate codes, every currency has required fields
	seen := map[string]bool{}
	for _, c := range SupportedCurrencies {
		if c.Code == "" {
			t.Errorf("currency missing Code: %+v", c)
		}
		if c.Name == "" {
			t.Errorf("currency %s missing Name", c.Code)
		}
		if c.Symbol == "" {
			t.Errorf("currency %s missing Symbol", c.Code)
		}
		if seen[c.Code] {
			t.Errorf("duplicate currency code: %s", c.Code)
		}
		seen[c.Code] = true
	}
	// DefaultCurrency must be in the supported list
	if !seen[DefaultCurrency] {
		t.Errorf("DefaultCurrency %q not in SupportedCurrencies", DefaultCurrency)
	}
}

func TestUSStatesIntegrity(t *testing.T) {
	// 50 states, no duplicates, no empty names
	if len(USStates) != 50 {
		t.Errorf("expected 50 US states, got %d", len(USStates))
	}
	seen := map[string]bool{}
	for _, s := range USStates {
		if s.Name == "" {
			t.Error("found empty state name")
		}
		if seen[s.Name] {
			t.Errorf("duplicate state: %s", s.Name)
		}
		seen[s.Name] = true
	}
	// DefaultCompanyState must be in the list
	if !seen[DefaultCompanyState] {
		t.Errorf("DefaultCompanyState %q not in USStates", DefaultCompanyState)
	}
}
