package catalog

import "strings"

type Currency struct {
	Code   string
	Name   string
	Symbol string
}

var SupportedCurrencies = []Currency{
	{Code: "USD", Name: "US Dollar", Symbol: "$"},
	{Code: "CAD", Name: "Canadian Dollar", Symbol: "CA$"},
	{Code: "GBP", Name: "British Pound", Symbol: "£"},
	{Code: "EUR", Name: "Euro", Symbol: "€"},
	{Code: "MXN", Name: "Mexican Peso", Symbol: "MX$"},
}

const (
	DefaultCurrency       = "USD"
	DefaultCompanyCountry = "US"
	DefaultCompanyState   = "California"
	DefaultClientCountry  = "US"
	DefaultClientState    = "California"
)

func NormalizeCurrency(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	for _, c := range SupportedCurrencies {
		if c.Code == code {
			return c.Code
		}
	}
	return DefaultCurrency
}

func CurrencySymbol(code string) string {
	code = NormalizeCurrency(code)
	for _, c := range SupportedCurrencies {
		if c.Code == code {
			return c.Symbol
		}
	}
	return "$"
}

func DefaultIfBlank(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
