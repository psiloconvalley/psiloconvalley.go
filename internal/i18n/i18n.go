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

	// Onboarding
	OnboardingTitle       string
	OnboardingStepsOf     string
	OnboardingAccount     string
	OnboardingAccountSub  string
	OnboardingProfile     string
	OnboardingProfileSub  string
	OnboardingClient      string
	OnboardingClientSub   string
	OnboardingInvoice     string
	OnboardingInvoiceSub  string
	OnboardingSetUp       string
	OnboardingAddClient   string
	OnboardingCreateInv   string

	// Dashboard extras
	InvoicesTotal         string
	ViewExpenses          string
	RevenueMinusExpenses  string
	CreateFirstInvoice    string
	CreateFirstEstimate   string
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

	OnboardingTitle:       "Getting Started",
	OnboardingStepsOf:     "of 4 steps complete",
	OnboardingAccount:     "Account Created",
	OnboardingAccountSub:  "You're in. Welcome to PsiloConValley.",
	OnboardingProfile:     "Set Up Business Profile",
	OnboardingProfileSub:  "Your name, logo, and contact info appear on every invoice.",
	OnboardingClient:      "Add Your First Client",
	OnboardingClientSub:   "Save client details once. Reuse on every invoice.",
	OnboardingInvoice:     "Send Your First Invoice",
	OnboardingInvoiceSub:  "Create, send, and get paid. Under 60 seconds.",
	OnboardingSetUp:       "Set Up →",
	OnboardingAddClient:   "Add Client →",
	OnboardingCreateInv:   "Create Invoice →",

	InvoicesTotal:         "invoices total",
	ViewExpenses:          "View all expenses →",
	RevenueMinusExpenses:  "Revenue minus expenses",
	CreateFirstInvoice:    "Create your first invoice to get started.",
	CreateFirstEstimate:   "Create your first estimate to start winning jobs.",
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
	DashboardRecentEstimates: "Presuestos Recientes",
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

	OnboardingTitle:       "Primeros Pasos",
	OnboardingStepsOf:     "de 4 pasos completados",
	OnboardingAccount:     "Cuenta Creada",
	OnboardingAccountSub:  "Ya estás dentro. Bienvenido a PsiloConValley.",
	OnboardingProfile:     "Configura Tu Perfil",
	OnboardingProfileSub:  "Tu nombre, logo e información de contacto aparecen en cada factura.",
	OnboardingClient:      "Agrega Tu Primer Cliente",
	OnboardingClientSub:   "Guarda los datos del cliente una vez. Reutilízalos en cada factura.",
	OnboardingInvoice:     "Envía Tu Primera Factura",
	OnboardingInvoiceSub:  "Crea, envía y cobra. En menos de 60 segundos.",
	OnboardingSetUp:       "Configurar →",
	OnboardingAddClient:   "Agregar Cliente →",
	OnboardingCreateInv:   "Crear Factura →",

	InvoicesTotal:         "facturas en total",
	ViewExpenses:          "Ver todos los gastos →",
	RevenueMinusExpenses:  "Ingresos menos gastos",
	CreateFirstInvoice:    "Crea tu primera factura para comenzar.",
	CreateFirstEstimate:   "Crea tu primer presupuesto para empezar a ganar trabajos.",
}

// Get returns the translation set for the given language code.
// Falls back to English for any unknown code.
func Get(lang string) T {
	if lang == "es" {
		return es
	}
	return en
}
