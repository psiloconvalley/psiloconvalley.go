// internal/i18n/i18n.go
// Simple translation system — English and Spanish.
// No external dependencies. Just maps.
package i18n

// T holds all translatable strings for the UI.
type T struct {
	// Nav
	NavDashboard string
	NavBilling   string
	NavInvoices  string
	NavEstimates string
	NavRecurring string
	NavClients   string
	NavPricing   string
	NavProfile   string
	NavLogout    string

	// Dashboard
	DashboardTitle    string
	DashboardWelcome  string
	DashboardRevenue  string
	DashboardOutstanding string
	DashboardOverdue  string
	DashboardThisMonth string
	DashboardNeedsAttention string
	DashboardAllCaughtUp string
	DashboardNoCaughtUpSub string
	DashboardRecentInvoices string
	DashboardRecentEstimates string
	DashboardViewAll string

	// Invoices
	InvoiceNew    string
	InvoiceDraft  string
	InvoiceSent   string
	InvoicePaid   string
	InvoiceVoid   string
	InvoiceOverdue string

	// Estimates
	EstimateNew      string
	EstimateDraft    string
	EstimateSent     string
	EstimateAccepted string
	EstimateDeclined string

	// Actions
	ActionSave   string
	ActionCancel string
	ActionDelete string
	ActionEdit   string
	ActionSend   string

	// Common
	NoInvoicesYet   string
	NoEstimatesYet  string
	NoClientsYet    string
	FromPaidInvoices string
	AwaitingPayment  string
	PastDueDate      string
}

var en = T{
	NavDashboard: "Dashboard",
	NavBilling:   "Billing",
	NavInvoices:  "Invoices",
	NavEstimates: "Estimates",
	NavRecurring: "Recurring",
	NavClients:   "Clients",
	NavPricing:   "Pricing",
	NavProfile:   "Profile",
	NavLogout:    "Logout",

	DashboardTitle:          "Command Center",
	DashboardWelcome:        "Welcome back",
	DashboardRevenue:        "Total Revenue",
	DashboardOutstanding:    "Outstanding",
	DashboardOverdue:        "Overdue",
	DashboardThisMonth:      "This Month",
	DashboardNeedsAttention: "Needs Attention",
	DashboardAllCaughtUp:    "You're all caught up",
	DashboardNoCaughtUpSub:  "No overdue invoices or pending estimates.",
	DashboardRecentInvoices: "Recent Invoices",
	DashboardRecentEstimates: "Recent Estimates",
	DashboardViewAll:        "View all →",

	InvoiceNew:     "New Invoice",
	InvoiceDraft:   "Draft",
	InvoiceSent:    "Sent",
	InvoicePaid:    "Paid",
	InvoiceVoid:    "Void",
	InvoiceOverdue: "Overdue",

	EstimateNew:      "New Estimate",
	EstimateDraft:    "Draft",
	EstimateSent:     "Sent",
	EstimateAccepted: "Accepted",
	EstimateDeclined: "Declined",

	ActionSave:   "Save",
	ActionCancel: "Cancel",
	ActionDelete: "Delete",
	ActionEdit:   "Edit",
	ActionSend:   "Send",

	NoInvoicesYet:    "No invoices yet",
	NoEstimatesYet:   "No estimates yet",
	NoClientsYet:     "No clients yet",
	FromPaidInvoices: "From paid invoices",
	AwaitingPayment:  "Awaiting payment",
	PastDueDate:      "Past due date",
}

var es = T{
	NavDashboard: "Panel",
	NavBilling:   "Facturación",
	NavInvoices:  "Facturas",
	NavEstimates: "Presupuestos",
	NavRecurring: "Recurrente",
	NavClients:   "Clientes",
	NavPricing:   "Precios",
	NavProfile:   "Perfil",
	NavLogout:    "Cerrar Sesión",

	DashboardTitle:          "Centro de Mando",
	DashboardWelcome:        "Bienvenido",
	DashboardRevenue:        "Ingresos Totales",
	DashboardOutstanding:    "Por Cobrar",
	DashboardOverdue:        "Vencido",
	DashboardThisMonth:      "Este Mes",
	DashboardNeedsAttention: "Requiere Atención",
	DashboardAllCaughtUp:    "Estás al día",
	DashboardNoCaughtUpSub:  "No hay facturas vencidas ni presupuestos pendientes.",
	DashboardRecentInvoices: "Facturas Recientes",
	DashboardRecentEstimates: "Presupuestos Recientes",
	DashboardViewAll:        "Ver todo →",

	InvoiceNew:     "Nueva Factura",
	InvoiceDraft:   "Borrador",
	InvoiceSent:    "Enviado",
	InvoicePaid:    "Pagado",
	InvoiceVoid:    "Anulado",
	InvoiceOverdue: "Vencido",

	EstimateNew:      "Nuevo Presupuesto",
	EstimateDraft:    "Borrador",
	EstimateSent:     "Enviado",
	EstimateAccepted: "Aceptado",
	EstimateDeclined: "Rechazado",

	ActionSave:   "Guardar",
	ActionCancel: "Cancelar",
	ActionDelete: "Eliminar",
	ActionEdit:   "Editar",
	ActionSend:   "Enviar",

	NoInvoicesYet:    "Aún no hay facturas",
	NoEstimatesYet:   "Aún no hay presupuestos",
	NoClientsYet:     "Aún no hay clientes",
	FromPaidInvoices: "De facturas pagadas",
	AwaitingPayment:  "Esperando pago",
	PastDueDate:      "Fecha vencida",
}

// Get returns the translation set for the given language code.
// Falls back to English for any unknown code.
func Get(lang string) T {
	if lang == "es" {
		return es
	}
	return en
}
