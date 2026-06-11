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

type USState struct {
	Name string
}

var USStates = []USState{
	{"Alabama"}, {"Alaska"}, {"Arizona"}, {"Arkansas"},
	{"California"}, {"Colorado"}, {"Connecticut"},
	{"Delaware"}, {"Florida"}, {"Georgia"},
	{"Hawaii"}, {"Idaho"}, {"Illinois"}, {"Indiana"}, {"Iowa"},
	{"Kansas"}, {"Kentucky"}, {"Louisiana"},
	{"Maine"}, {"Maryland"}, {"Massachusetts"}, {"Michigan"},
	{"Minnesota"}, {"Mississippi"}, {"Missouri"}, {"Montana"},
	{"Nebraska"}, {"Nevada"}, {"New Hampshire"}, {"New Jersey"},
	{"New Mexico"}, {"New York"}, {"North Carolina"}, {"North Dakota"},
	{"Ohio"}, {"Oklahoma"}, {"Oregon"},
	{"Pennsylvania"}, {"Rhode Island"},
	{"South Carolina"}, {"South Dakota"},
	{"Tennessee"}, {"Texas"}, {"Utah"},
	{"Vermont"}, {"Virginia"},
	{"Washington"}, {"West Virginia"}, {"Wisconsin"}, {"Wyoming"},
}


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
// NormalizeState maps US state abbreviations and mixed-case input
// to the canonical full name used in USStates.
// Returns input unchanged if no match found — safe for non-US addresses.
func NormalizeState(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return input
	}

	abbrevs := map[string]string{
		"AL": "Alabama", "AK": "Alaska", "AZ": "Arizona", "AR": "Arkansas",
		"CA": "California", "CO": "Colorado", "CT": "Connecticut", "DE": "Delaware",
		"FL": "Florida", "GA": "Georgia", "HI": "Hawaii", "ID": "Idaho",
		"IL": "Illinois", "IN": "Indiana", "IA": "Iowa", "KS": "Kansas",
		"KY": "Kentucky", "LA": "Louisiana", "ME": "Maine", "MD": "Maryland",
		"MA": "Massachusetts", "MI": "Michigan", "MN": "Minnesota", "MS": "Mississippi",
		"MO": "Missouri", "MT": "Montana", "NE": "Nebraska", "NV": "Nevada",
		"NH": "New Hampshire", "NJ": "New Jersey", "NM": "New Mexico", "NY": "New York",
		"NC": "North Carolina", "ND": "North Dakota", "OH": "Ohio", "OK": "Oklahoma",
		"OR": "Oregon", "PA": "Pennsylvania", "RI": "Rhode Island", "SC": "South Carolina",
		"SD": "South Dakota", "TN": "Tennessee", "TX": "Texas", "UT": "Utah",
		"VT": "Vermont", "VA": "Virginia", "WA": "Washington", "WV": "West Virginia",
		"WI": "Wisconsin", "WY": "Wyoming",
	}

	// Check abbreviation first (e.g. "CA", "ca", "Ca")
	if full, ok := abbrevs[strings.ToUpper(input)]; ok {
		return full
	}

	// Check full name case-insensitively (e.g. "california", "CALIFORNIA")
	for _, s := range USStates {
		if strings.EqualFold(s.Name, input) {
			return s.Name
		}
	}

	// Non-US or unknown — return as-is
	return input
}
