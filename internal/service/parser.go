package service

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseTaxRateBps converts a percentage string (e.g. "8.25") to basis points (825).
// Empty input returns 0 bps with no error.
// Inputs < 0 or > 100 return an error.
func ParseTaxRateBps(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	pct, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid tax rate %q: %w", raw, err)
	}
	if pct < 0 || pct > 100 {
		return 0, fmt.Errorf("tax rate must be between 0 and 100, got %v", pct)
	}
	return int64(math.Round(pct * 100)), nil
}

// ParseCurrencyCents converts a dollar/currency string (e.g. "125.50") to integer cents (12550).
// Empty input returns 0 cents with no error.
// Negative amounts return an error.
func ParseCurrencyCents(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	amt, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", raw, err)
	}
	if amt < 0 {
		return 0, fmt.Errorf("amount cannot be negative, got %v", amt)
	}
	return int64(math.Round(amt * 100)), nil
}
