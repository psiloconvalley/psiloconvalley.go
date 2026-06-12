package catalog

// ExpenseCategories is the authoritative list of expense categories.
// Used in the expense form dropdown and for validation on save.
var ExpenseCategories = []string{
	"Software & Subscriptions",
	"Travel & Transportation",
	"Meals & Entertainment",
	"Equipment & Hardware",
	"Contractors & Freelancers",
	"Marketing & Advertising",
	"Office & Supplies",
	"Legal & Professional",
	"Taxes & Fees",
	"Other",
}

// ValidExpenseCategory returns true if the given category is in the list.
func ValidExpenseCategory(cat string) bool {
	for _, c := range ExpenseCategories {
		if c == cat {
			return true
		}
	}
	return false
}
