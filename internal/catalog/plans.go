// internal/catalog/plans.go
// Canonical plan definitions for the entire application.
// Every file that needs to check a plan imports this — no string literals elsewhere.
//
// Pricing matrix (July 2026):
//   Feature          Free($0)    Pro($18.88)    Pro Max($28.88)
//   ──────────────────────────────────────────────────────────
//   Invoices         5/mo        30/mo          Unlimited
//   Estimates        3/mo        30/mo          Unlimited
//   Clients          5           30             Unlimited
//   Email sends      3/mo        30/mo          Unlimited
//   Stripe payments  ✗           ✓              ✓
//   Expenses         ✗           ✓              ✓
//   Reports          0           Unlimited      Unlimited
//   Recurring        ✗           ✗              ✓
//   Reminders        ✗           ✗              ✓
//   Adv templates    ✗           ✗              ✓
//   Biz profile      ✓           ✓              ✓
//   Google autocomplete ✓        ✓              ✓
package catalog

const (
	PlanFree   = "free"
	PlanPro    = "pro"
	PlanProMax = "promax"

	// Legacy plan — mapped to Pro in all new logic
	PlanGrowthLegacy = "growth"
)

// Plan limits
const (
	FreeInvoiceLimit  = 5
	FreeSendLimit     = 3
	FreeClientLimit   = 5
	FreeEstimateLimit = 3
	FreeReportLimit   = 0

	ProInvoiceLimit  = 30
	ProSendLimit     = 30
	ProClientLimit   = 30
	ProEstimateLimit = 30
	ProReportLimit   = 0 // unlimited (0 = no limit in usageAllowed)
)

// IsPro returns true for Pro plan users.
func IsPro(plan string) bool {
	return plan == PlanPro || plan == PlanGrowthLegacy
}

// IsProMax returns true for Pro Max plan users.
func IsProMax(plan string) bool {
	return plan == PlanProMax
}

// IsPaid returns true for any paid plan.
func IsPaid(plan string) bool {
	return IsPro(plan) || IsProMax(plan)
}

// IsUnlimited returns true if the user has unlimited access.
// Pro Max users always get unlimited.
// Grandfathered early users (ID <= 10) on Pro also get unlimited.
func IsUnlimited(userID int64, plan string) bool {
	if IsProMax(plan) {
		return true
	}
	// Grandfather clause: early Pro users keep unlimited access
	if IsPro(plan) && userID <= 10 {
		return true
	}
	return false
}

// HasAutomation returns true if the user can use recurring invoices and reminders.
func HasAutomation(userID int64, plan string) bool {
	return IsUnlimited(userID, plan)
}

// HasAdvancedTemplates returns true if the user can use premium invoice templates.
func HasAdvancedTemplates(userID int64, plan string) bool {
	return IsUnlimited(userID, plan)
}

// NormalizePlan maps legacy plan names to current ones.
// "growth" → "pro" for any new logic.
func NormalizePlan(plan string) string {
	if plan == PlanGrowthLegacy {
		return PlanPro
	}
	if plan == PlanPro || plan == PlanProMax || plan == PlanFree {
		return plan
	}
	return PlanFree
}
