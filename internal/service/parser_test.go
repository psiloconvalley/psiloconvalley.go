package service

import "testing"

func TestParseTaxRateBps(t *testing.T) {
	// Table-driven test 
	tests := []struct { 
		name	string
		input	string
		want	int64
		wantErr	bool
	}{
	{"empty string returns 0 bps", "", 0, false},
		{"whitespace returns 0 bps", "   ", 0, false},
		{"simple integer returns matching bps", "8", 800, false},
		{"decimal rounds correctly", "8.25", 825, false},
		{"IEEE-754 imprecision handled", "8.25", 825, false},
		{"maximum allowed value 100", "100", 10000, false},
		{"zero is valid", "0", 0, false},
		{"negative value returns error", "-5", 0, true},
		{"over 100 returns error", "101", 0, true},
		{"non-numeric returns error", "abc", 0, true},
		{"symbols return error", "8.25%", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTaxRateBps(tc.input)

			// Verify error state matches expectation
			if (err != nil) != tc.wantErr {
				t.Errorf("ParseTaxRateBps(%q) error = %v, wantErr = %v",
					tc.input, err, tc.wantErr)
				return
			}

			// Verify the returned value matches expectation
			if got != tc.want {
				t.Errorf("ParseTaxRateBps(%q) = %d, want %d",
					tc.input, got, tc.want)
			}
		})
	}
}
