// internal/util/money_test.go
package util

import "testing"

func TestDollarsToCents(t *testing.T) {
	tests := []struct {
		input float64
		want  int64
	}{
		{0.00, 0},
		{1.00, 100},
		{1.50, 150},
		{1.99, 199},
		{9.99, 999},
		{100.00, 10000},
		{0.01, 1},
		// rounding — this is why int64(x * 100) is wrong
		{1.005, 100},
		{2.675, 268},
	}

	for _, tt := range tests {
		got := DollarsToCents(tt.input)
		if got != tt.want {
			t.Errorf("DollarsToCents(%v) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestCentsToDollars(t *testing.T) {
	tests := []struct {
		input int64
		want  float64
	}{
		{0, 0.00},
		{100, 1.00},
		{150, 1.50},
		{199, 1.99},
		{999, 9.99},
		{10000, 100.00},
		{1, 0.01},
	}

	for _, tt := range tests {
		got := CentsToDollars(tt.input)
		if got != tt.want {
			t.Errorf("CentsToDollars(%v) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestParseCents(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"", 0},
		{"0", 0},
		{"1.00", 100},
		{"1.50", 150},
		{"9.99", 999},
		{"100.00", 10000},
		{"  1.50  ", 150},
		{"bad", 0},
	}

	for _, tt := range tests {
		got := ParseCents(tt.input)
		if got != tt.want {
			t.Errorf("ParseCents(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestMoney(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "$0.00"},
		{100, "$1.00"},
		{150, "$1.50"},
		{999, "$9.99"},
		{10000, "$100.00"},
	}

	for _, tt := range tests {
		got := Money(tt.input)
		if got != tt.want {
			t.Errorf("Money(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
