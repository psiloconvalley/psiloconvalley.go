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
	// Profile page
	ProfileTitle            string
	ProfileSubtitle         string
	ProfileWelcomeTitle     string
	ProfileWelcomeBody      string
	ProfileWelcomeStep      string
	ProfileWelcomeSkip      string
	ProfileSaved            string
	ProfilePasswordSaved    string
	ProfileMagicTitle       string
	ProfileMagicBody        string
	ProfileSectionLogo      string
	ProfileLogoHint         string
	ProfileSectionCompany   string
	ProfileSectionAddress   string
	ProfileSectionFinancial string
	ProfileSectionPayment   string
	ProfileSectionLanguage  string
	ProfileCompanyName      string
	ProfileEmail            string
	ProfileStreetAddress    string
	ProfileCity             string
	ProfileStateRegion      string
	ProfileZipPostal        string
	ProfileCountry          string
	ProfileTaxID            string
	ProfileTaxIDHint        string
	ProfileCurrency         string
	ProfileStripeConnected    string
	ProfileStripeConnectedSub string
	ProfileStripeError        string
	ProfileStripeAcceptCards    string
	ProfileStripeAcceptCardsSub string
	ProfileStripeConnect       string
	ProfileSave             string
	ProfileChangePassword   string
	ProfileSetPassword      string
	ProfileCurrentPassword  string
	ProfileNewPassword      string
	ProfileConfirmPassword  string
	ProfilePasswordMin      string
	ProfilePasswordRepeat   string
	ProfileUpdatePassword   string
	ProfileErrCurrentPW     string
	ProfileErrShortPW       string
	ProfileErrMismatchPW    string
	ProfileErrFailedPW      string
	ProfileGoogleSignIn     string
	ProfilePreviewLabel     string
	ProfilePreviewLogo      string
	ProfilePreviewCompany   string
	ProfilePreviewInvoice   string
	ProfilePreviewItem1     string
	ProfilePreviewItem2     string
	ProfilePreviewItem3     string
	ProfilePreviewTotal     string
	ProfilePreviewHint      string
	ProfilePreviewMobile    string
	// Clients page
	ClientsTitle          string
	ClientsSaved          string
	ClientsProUnlimited   string
	ClientsFreeSlotsUsed  string
	ClientsNewClient      string
	ClientsLimitBanner    string
	ClientsLimitUpgrade   string
	ClientsSavedSuccess   string
	ClientsThClient       string
	ClientsThLocation     string
	ClientsThPhone        string
	ClientsThOperations   string
	ClientsEdit           string
	ClientsInvoice        string
	ClientsDelete         string
	ClientsDeleteConfirm  string
	ClientsEmptyTitle     string
	ClientsEmptyBody      string
	ClientsAddFirst       string
	// Client form
	ClientFormNewTitle       string
	ClientFormEditTitle      string
	ClientFormNewSubtitle    string
	ClientFormEditSubtitle   string
	ClientFormSectionContact string
	ClientFormClientName     string
	ClientFormEmail          string
	ClientFormPhone          string
	ClientFormSectionAddress string
	ClientFormStreetAddress  string
	ClientFormCity           string
	ClientFormStateRegion    string
	ClientFormZipPostal      string
	ClientFormCountry        string
	ClientFormSectionNotes   string
	ClientFormInternalNotes  string
	ClientFormNotesPlaceholder string
	ClientFormCancel         string
	ClientFormSaveChanges    string
	ClientFormSaveClient     string
	// Invoices list
	InvoicesTitle           string
	InvoicesListTotal       string
	InvoicesProUnlimited    string
	InvoicesThisMonth       string
	InvoicesNewInvoice      string
	InvoicesDeleted         string
	InvoicesMonthlyUsage    string
	InvoicesUsageLabel      string
	InvoicesUpgrade         string
	InvoicesEmptyTitle      string
	InvoicesEmptyBody       string
	InvoicesCreateInvoice   string
	InvoicesThInvoice       string
	InvoicesThClient        string
	InvoicesThIssued        string
	InvoicesThDue           string
	InvoicesThStatus        string
	InvoicesThTotal         string
	InvoicesThActions       string
	InvoicesSend            string
	InvoicesEdit            string
	InvoicesResend          string
	InvoicesDuplicate       string
	InvoicesDeleteBtn       string
	InvoicesUntitled        string
	InvoicesNoClient        string
	InvoicesIssuedLabel     string
	InvoicesDueLabel        string
	InvoicesDeleteTitle     string
	InvoicesDeleteBody      string
	InvoicesDeleteConfirm   string
	InvoicesDeletePlaceholder string
	InvoicesDeleteCancel    string
	InvoicesDeletePermanent string

	// Estimates list page
	EstimatesListTitle           string
	EstimatesListTotal           string
	EstimatesListNew             string
	EstimatesListDeleted         string
	EstimatesListConverted       string
	EstimatesListEmptyTitle      string
	EstimatesListEmptySub        string
	EstimatesListEmptyBtn        string
	EstimatesListThEstimate      string
	EstimatesListThClient        string
	EstimatesListThIssued        string
	EstimatesListThValidUntil    string
	EstimatesListThStatus        string
	EstimatesListThTotal         string
	EstimatesListThActions       string
	EstimatesListSend            string
	EstimatesListEdit            string
	EstimatesListResend          string
	EstimatesListView            string
	EstimatesListConvert         string
	EstimatesListDeleteBtn       string
	EstimatesListUntitled        string
	EstimatesListNoClient        string
	EstimatesListCreatedLabel    string
	EstimatesListDeleteTitle     string
	EstimatesListDeleteBody      string
	EstimatesListDeleteConfirm   string
	EstimatesListDeletePlaceholder string
	EstimatesListDeleteCancel    string
	EstimatesListDeletePermanent string
	EstimatesListAcceptedTitle   string
	EstimatesListAcceptedBody    string
	EstimatesListConvertToInv    string
	EstimatesListConvertSub      string
	EstimatesListEditAnyway      string
	EstimatesListEditAnywayWarn  string
	EstimatesListAcceptedCancel  string

	// Login page
	LoginTitle           string
	LoginSubtitle        string
	LoginLimitBanner     string
	LoginEmail           string
	LoginPassword        string
	LoginSubmit          string
	LoginForgot          string
	LoginOr              string
	LoginGoogle          string
	LoginGoogleSoon      string
	LoginNoAccount       string
	LoginCreateOne       string

	// Register page
	RegisterTitle        string
	RegisterSubtitle     string
	RegisterEmail        string
	RegisterPassword     string
	RegisterPasswordHint string
	RegisterConfirm      string
	RegisterSubmit       string
	RegisterOr           string
	RegisterGoogle       string
	RegisterGoogleSoon   string
	RegisterHaveAccount  string
	RegisterSignIn       string
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
		ProfileTitle:            "Business Profile",
	ProfileSubtitle:         "Design your invoice identity. Changes appear instantly in the preview.",
	ProfileWelcomeTitle:     "Welcome to PsiloConValley",
	ProfileWelcomeBody:      "Your account is ready. Start by filling out your business profile below — your name, logo, and contact info will appear on every invoice you send. This takes about 30 seconds.",
	ProfileWelcomeStep:      "✓ Step 1 of 4 complete",
	ProfileWelcomeSkip:      "Skip to Dashboard →",
	ProfileSaved:            "✓ Profile saved successfully",
	ProfilePasswordSaved:    "✓ Password updated successfully. You can now sign in with your new password.",
	ProfileMagicTitle:       "You're signed in via magic link",
	ProfileMagicBody:        "Set a password below so you can sign in with email next time. This is optional — you can always request another sign-in link.",
	ProfileSectionLogo:      "Logo",
	ProfileLogoHint:         "PNG recommended. Max 2MB.",
	ProfileSectionCompany:   "Company Information",
	ProfileSectionAddress:   "Address",
	ProfileSectionFinancial: "Financial Settings",
	ProfileSectionPayment:   "Payment Processing",
	ProfileSectionLanguage:  "Language / Idioma",
	ProfileCompanyName:      "Company Name *",
	ProfileEmail:            "Email Address",
	ProfileStreetAddress:    "Street Address",
	ProfileCity:             "City",
	ProfileStateRegion:      "State / Region",
	ProfileZipPostal:        "Zip / Postal Code",
	ProfileCountry:          "Country",
	ProfileTaxID:            "Tax ID / EIN",
	ProfileTaxIDHint:        "Optional. Appears on invoices if provided.",
	ProfileCurrency:         "Default Currency",
	ProfileStripeConnected:     "Stripe Connected",
	ProfileStripeConnectedSub:  "Clients can pay by card.",
	ProfileStripeError:         "Could not connect Stripe account. Please try again.",
	ProfileStripeAcceptCards:    "Accept Card Payments",
	ProfileStripeAcceptCardsSub: "Connect Stripe so clients can pay invoices directly.",
	ProfileStripeConnect:       "Connect Stripe",
	ProfileSave:             "Save Profile",
	ProfileChangePassword:   "Change Password",
	ProfileSetPassword:      "Set a Password",
	ProfileCurrentPassword:  "Current Password",
	ProfileNewPassword:      "New Password",
	ProfileConfirmPassword:  "Confirm Password",
	ProfilePasswordMin:      "Min. 8 characters",
	ProfilePasswordRepeat:   "Repeat new password",
	ProfileUpdatePassword:   "Update Password",
	ProfileErrCurrentPW:     "Current password is incorrect.",
	ProfileErrShortPW:       "New password must be at least 8 characters.",
	ProfileErrMismatchPW:    "Passwords do not match.",
	ProfileErrFailedPW:      "Could not update password. Please try again.",
	ProfileGoogleSignIn:     "Your account uses Google sign-in. Setting a password lets you also sign in with email.",
	ProfilePreviewLabel:     "Invoice Preview",
	ProfilePreviewLogo:      "YOUR LOGO",
	ProfilePreviewCompany:   "Your Company",
	ProfilePreviewInvoice:   "INVOICE",
	ProfilePreviewItem1:     "Pool Cleaning Service",
	ProfilePreviewItem2:     "Chemical Treatment",
	ProfilePreviewItem3:     "Equipment Check",
	ProfilePreviewTotal:     "Total",
	ProfilePreviewHint:      "Updates live as you type",
	ProfilePreviewMobile:    "👁 Preview Invoice",
	ClientsTitle:          "Clients",
	ClientsSaved:          "saved",
	ClientsProUnlimited:   "Pro — Unlimited",
	ClientsFreeSlotsUsed:  "free slots used",
	ClientsNewClient:      "+ New Client",
	ClientsLimitBanner:    "You've used all 5 free client slots. Upgrade to Pro for unlimited clients, invoice history per client, and revenue analytics.",
	ClientsLimitUpgrade:   "Upgrade to Pro →",
	ClientsSavedSuccess:   "✓ Client saved successfully",
	ClientsThClient:       "Client",
	ClientsThLocation:     "Location",
	ClientsThPhone:        "Phone",
	ClientsThOperations:   "Operations",
	ClientsEdit:           "Edit",
	ClientsInvoice:        "Invoice",
	ClientsDelete:         "Delete",
	ClientsDeleteConfirm:  "Delete this client? This cannot be undone.",
	ClientsEmptyTitle:     "No Clients Saved",
	ClientsEmptyBody:      "Save your first client and their info will auto-fill every future invoice. No more retyping addresses.",
	ClientsAddFirst:       "Add First Client",
	ClientFormNewTitle:       "New Client",
	ClientFormEditTitle:      "Edit Client",
	ClientFormNewSubtitle:    "Saved clients auto-fill on every new invoice.",
	ClientFormEditSubtitle:   "Update client information. Changes apply to future invoices.",
	ClientFormSectionContact: "Contact Information",
	ClientFormClientName:     "Client Name *",
	ClientFormEmail:          "Email Address",
	ClientFormPhone:          "Phone",
	ClientFormSectionAddress: "Address",
	ClientFormStreetAddress:  "Street Address",
	ClientFormCity:           "City",
	ClientFormStateRegion:    "State / Region",
	ClientFormZipPostal:      "Zip / Postal",
	ClientFormCountry:        "Country",
	ClientFormSectionNotes:   "Notes",
	ClientFormInternalNotes:  "Internal Notes",
	ClientFormNotesPlaceholder: "Payment terms, preferences, contact details...",
	ClientFormCancel:         "Cancel",
	ClientFormSaveChanges:    "Save Changes",
	ClientFormSaveClient:     "Save Client",
	InvoicesTitle:           "Invoices",
	InvoicesListTotal:           "total",
	InvoicesProUnlimited:    "Pro — Unlimited",
	InvoicesThisMonth:       "this month",
	InvoicesNewInvoice:      "+ New Invoice",
	InvoicesDeleted:         "✓ Invoice deleted successfully",
	InvoicesMonthlyUsage:    "Monthly Usage",
	InvoicesUsageLabel:      "invoices",
	InvoicesUpgrade:         "Upgrade →",
	InvoicesEmptyTitle:      "No invoices yet",
	InvoicesEmptyBody:       "Create your first invoice to get started.",
	InvoicesCreateInvoice:   "Create Invoice",
	InvoicesThInvoice:       "Invoice",
	InvoicesThClient:        "Client",
	InvoicesThIssued:        "Issued",
	InvoicesThDue:           "Due",
	InvoicesThStatus:        "Status",
	InvoicesThTotal:         "Total",
	InvoicesThActions:       "Actions",
	InvoicesSend:            "Send",
	InvoicesEdit:            "Edit",
	InvoicesResend:          "Resend",
	InvoicesDuplicate:       "Duplicate",
	InvoicesDeleteBtn:       "Delete",
	InvoicesUntitled:        "Untitled",
	InvoicesNoClient:        "No client",
	InvoicesIssuedLabel:     "Issued",
	InvoicesDueLabel:        "Due",
	InvoicesDeleteTitle:     "Delete Draft Invoice",
	InvoicesDeleteBody:      "This action cannot be undone. The invoice and all its line items will be permanently deleted.",
	InvoicesDeleteConfirm:   "Type the invoice number to confirm:",
	InvoicesDeletePlaceholder: "Type invoice number here",
	InvoicesDeleteCancel:    "Cancel",
	InvoicesDeletePermanent: "Delete Permanently",

	EstimatesListTitle:            "Estimates",
	EstimatesListTotal:            "total",
	EstimatesListNew:              "+ New Estimate",
	EstimatesListDeleted:          "✓ Estimate deleted successfully",
	EstimatesListConverted:        "✓ Estimate converted to invoice",
	EstimatesListEmptyTitle:       "No estimates yet",
	EstimatesListEmptySub:         "Create your first estimate to send to a client for approval.",
	EstimatesListEmptyBtn:         "Create Estimate",
	EstimatesListThEstimate:       "Estimate",
	EstimatesListThClient:         "Client",
	EstimatesListThIssued:         "Issued",
	EstimatesListThValidUntil:     "Valid Until",
	EstimatesListThStatus:         "Status",
	EstimatesListThTotal:          "Total",
	EstimatesListThActions:        "Actions",
	EstimatesListSend:             "Send",
	EstimatesListEdit:             "Edit",
	EstimatesListResend:           "Resend",
	EstimatesListView:             "View",
	EstimatesListConvert:          "Convert",
	EstimatesListDeleteBtn:        "Delete",
	EstimatesListUntitled:         "Untitled",
	EstimatesListNoClient:         "No client",
	EstimatesListCreatedLabel:     "Created",
	EstimatesListDeleteTitle:      "Delete Draft Estimate",
	EstimatesListDeleteBody:       "This action cannot be undone. The estimate and all its line items will be permanently deleted.",
	EstimatesListDeleteConfirm:    "Type the estimate number to confirm:",
	EstimatesListDeletePlaceholder: "Type estimate number here",
	EstimatesListDeleteCancel:     "Cancel",
	EstimatesListDeletePermanent:  "Delete Permanently",
	EstimatesListAcceptedTitle:    "This estimate has been accepted",
	EstimatesListAcceptedBody:     "The client has already approved this estimate. What would you like to do?",
	EstimatesListConvertToInv:     "Convert to Invoice",
	EstimatesListConvertSub:       "Recommended — create an invoice from this accepted estimate",
	EstimatesListEditAnyway:       "Edit Anyway",
	EstimatesListEditAnywayWarn:   "Status will revert to draft — client will need to re-accept",
	EstimatesListAcceptedCancel:   "Cancel",

	LoginTitle:           "Welcome back",
	LoginSubtitle:        "Sign in to your account",
	LoginLimitBanner:     "You've used your free invoices. Sign in or create an account to continue — it's free.",
	LoginEmail:           "Email",
	LoginPassword:        "Password",
	LoginSubmit:          "Sign In",
	LoginForgot:          "Forgot password?",
	LoginOr:              "or",
	LoginGoogle:          "Continue with Google",
	LoginGoogleSoon:      "Coming Soon",
	LoginNoAccount:       "Don't have an account?",
	LoginCreateOne:       "Create one",

	RegisterTitle:        "Create your account",
	RegisterSubtitle:     "Generate invoices. Track payments. Own your data.",
	RegisterEmail:        "Email",
	RegisterPassword:     "Password",
	RegisterPasswordHint: "Minimum 8 characters",
	RegisterConfirm:      "Confirm Password",
	RegisterSubmit:       "Create Account",
	RegisterOr:           "or",
	RegisterGoogle:       "Continue with Google",
	RegisterGoogleSoon:   "Coming Soon",
	RegisterHaveAccount:  "Already have an account?",
	RegisterSignIn:       "Sign in",

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
	ProfileTitle:            "Perfil de Negocio",
	ProfileSubtitle:         "Diseña la identidad de tus facturas. Los cambios aparecen al instante en la vista previa.",
	ProfileWelcomeTitle:     "Bienvenido a PsiloConValley",
	ProfileWelcomeBody:      "Tu cuenta está lista. Comienza llenando tu perfil de negocio — tu nombre, logo e información de contacto aparecerán en cada factura que envíes. Toma unos 30 segundos.",
	ProfileWelcomeStep:      "✓ Paso 1 de 4 completado",
	ProfileWelcomeSkip:      "Ir al Panel →",
	ProfileSaved:            "✓ Perfil guardado exitosamente",
	ProfilePasswordSaved:    "✓ Contraseña actualizada exitosamente. Ya puedes iniciar sesión con tu nueva contraseña.",
	ProfileMagicTitle:       "Ingresaste con enlace mágico",
	ProfileMagicBody:        "Establece una contraseña para poder iniciar sesión con tu correo la próxima vez. Esto es opcional — siempre puedes solicitar otro enlace.",
	ProfileSectionLogo:      "Logo",
	ProfileLogoHint:         "PNG recomendado. Máx 2MB.",
	ProfileSectionCompany:   "Información de la Empresa",
	ProfileSectionAddress:   "Dirección",
	ProfileSectionFinancial: "Configuración Financiera",
	ProfileSectionPayment:   "Procesamiento de Pagos",
	ProfileSectionLanguage:  "Language / Idioma",
	ProfileCompanyName:      "Nombre de Empresa *",
	ProfileEmail:            "Correo Electrónico",
	ProfileStreetAddress:    "Dirección",
	ProfileCity:             "Ciudad",
	ProfileStateRegion:      "Estado / Región",
	ProfileZipPostal:        "Código Postal",
	ProfileCountry:          "País",
	ProfileTaxID:            "RFC / ID Fiscal",
	ProfileTaxIDHint:        "Opcional. Aparece en las facturas si se proporciona.",
	ProfileCurrency:         "Moneda Predeterminada",
	ProfileStripeConnected:     "Stripe Conectado",
	ProfileStripeConnectedSub:  "Los clientes pueden pagar con tarjeta.",
	ProfileStripeError:         "No se pudo conectar la cuenta de Stripe. Intenta de nuevo.",
	ProfileStripeAcceptCards:    "Aceptar Pagos con Tarjeta",
	ProfileStripeAcceptCardsSub: "Conecta Stripe para que tus clientes paguen facturas directamente.",
	ProfileStripeConnect:       "Conectar Stripe",
	ProfileSave:             "Guardar Perfil",
	ProfileChangePassword:   "Cambiar Contraseña",
	ProfileSetPassword:      "Establecer Contraseña",
	ProfileCurrentPassword:  "Contraseña Actual",
	ProfileNewPassword:      "Nueva Contraseña",
	ProfileConfirmPassword:  "Confirmar Contraseña",
	ProfilePasswordMin:      "Mín. 8 caracteres",
	ProfilePasswordRepeat:   "Repite la nueva contraseña",
	ProfileUpdatePassword:   "Actualizar Contraseña",
	ProfileErrCurrentPW:     "La contraseña actual es incorrecta.",
	ProfileErrShortPW:       "La nueva contraseña debe tener al menos 8 caracteres.",
	ProfileErrMismatchPW:    "Las contraseñas no coinciden.",
	ProfileErrFailedPW:      "No se pudo actualizar la contraseña. Intenta de nuevo.",
	ProfileGoogleSignIn:     "Tu cuenta usa inicio de sesión con Google. Establecer una contraseña te permite también iniciar sesión con correo.",
	ProfilePreviewLabel:     "Vista Previa de Factura",
	ProfilePreviewLogo:      "TU LOGO",
	ProfilePreviewCompany:   "Tu Empresa",
	ProfilePreviewInvoice:   "FACTURA",
	ProfilePreviewItem1:     "Servicio de Limpieza de Piscina",
	ProfilePreviewItem2:     "Tratamiento Químico",
	ProfilePreviewItem3:     "Revisión de Equipo",
	ProfilePreviewTotal:     "Total",
	ProfilePreviewHint:      "Se actualiza mientras escribes",
	ProfilePreviewMobile:    "👁 Vista Previa",
	ClientsTitle:          "Clientes",
	ClientsSaved:          "guardados",
	ClientsProUnlimited:   "Pro — Ilimitado",
	ClientsFreeSlotsUsed:  "espacios gratis usados",
	ClientsNewClient:      "+ Nuevo Cliente",
	ClientsLimitBanner:    "Has usado los 5 espacios gratis. Actualiza a Pro para clientes ilimitados, historial de facturas por cliente y análisis de ingresos.",
	ClientsLimitUpgrade:   "Actualizar a Pro →",
	ClientsSavedSuccess:   "✓ Cliente guardado exitosamente",
	ClientsThClient:       "Cliente",
	ClientsThLocation:     "Ubicación",
	ClientsThPhone:        "Teléfono",
	ClientsThOperations:   "Acciones",
	ClientsEdit:           "Editar",
	ClientsInvoice:        "Factura",
	ClientsDelete:         "Eliminar",
	ClientsDeleteConfirm:  "¿Eliminar este cliente? Esta acción no se puede deshacer.",
	ClientsEmptyTitle:     "No Hay Clientes Guardados",
	ClientsEmptyBody:      "Guarda tu primer cliente y su información se llenará automáticamente en cada factura. No más escribir direcciones.",
	ClientsAddFirst:       "Agregar Primer Cliente",
	ClientFormNewTitle:       "Nuevo Cliente",
	ClientFormEditTitle:      "Editar Cliente",
	ClientFormNewSubtitle:    "Los clientes guardados se llenan automáticamente en cada nueva factura.",
	ClientFormEditSubtitle:   "Actualiza la información del cliente. Los cambios aplican a futuras facturas.",
	ClientFormSectionContact: "Información de Contacto",
	ClientFormClientName:     "Nombre del Cliente *",
	ClientFormEmail:          "Correo Electrónico",
	ClientFormPhone:          "Teléfono",
	ClientFormSectionAddress: "Dirección",
	ClientFormStreetAddress:  "Dirección",
	ClientFormCity:           "Ciudad",
	ClientFormStateRegion:    "Estado / Región",
	ClientFormZipPostal:      "Código Postal",
	ClientFormCountry:        "País",
	ClientFormSectionNotes:   "Notas",
	ClientFormInternalNotes:  "Notas Internas",
	ClientFormNotesPlaceholder: "Términos de pago, preferencias, datos de contacto...",
	ClientFormCancel:        "Cancelar",
	ClientFormSaveChanges:   "Guardar Cambios",
	ClientFormSaveClient:    "Guardar Cliente",
	InvoicesTitle:           "Facturas",
	InvoicesListTotal:       "en total",
	InvoicesProUnlimited:    "Pro — Ilimitado",
	InvoicesThisMonth:       "este mes",
	InvoicesNewInvoice:      "+ Nueva Factura",
	InvoicesDeleted:         "✓ Factura eliminada exitosamente",
	InvoicesMonthlyUsage:    "Uso Mensual",
	InvoicesUsageLabel:      "facturas",
	InvoicesUpgrade:         "Actualizar →",
	InvoicesEmptyTitle:      "Aún no hay facturas",
	InvoicesEmptyBody:       "Crea tu primera factura para comenzar.",
	InvoicesCreateInvoice:   "Crear Factura",
	InvoicesThInvoice:       "Factura",
	InvoicesThClient:        "Cliente",
	InvoicesThIssued:        "Emitida",
	InvoicesThDue:           "Vence",
	InvoicesThStatus:        "Estado",
	InvoicesThTotal:         "Total",
	InvoicesThActions:       "Acciones",
	InvoicesSend:            "Enviar",
	InvoicesEdit:            "Editar",
	InvoicesResend:          "Reenviar",
	InvoicesDuplicate:       "Duplicar",
	InvoicesDeleteBtn:       "Eliminar",
	InvoicesUntitled:        "Sin Título",
	InvoicesNoClient:        "Sin cliente",
	InvoicesIssuedLabel:     "Emitida",
	InvoicesDueLabel:        "Vence",
	InvoicesDeleteTitle:     "Eliminar Borrador de Factura",
	InvoicesDeleteBody:      "Esta acción no se puede deshacer. La factura y todos sus artículos serán eliminados permanentemente.",
	InvoicesDeleteConfirm:   "Escribe el número de factura para confirmar:",
	InvoicesDeletePlaceholder: "Escribe el número de factura aquí",
	InvoicesDeleteCancel:    "Cancelar",
	InvoicesDeletePermanent: "Eliminar Permanentemente",

	EstimatesListTitle:            "Presupuestos",
	EstimatesListTotal:            "en total",
	EstimatesListNew:              "+ Nuevo Presupuesto",
	EstimatesListDeleted:          "✓ Presupuesto eliminado exitosamente",
	EstimatesListConverted:        "✓ Presupuesto convertido a factura",
	EstimatesListEmptyTitle:       "Aún no hay presupuestos",
	EstimatesListEmptySub:         "Crea tu primer presupuesto para enviarlo a un cliente para su aprobación.",
	EstimatesListEmptyBtn:         "Crear Presupuesto",
	EstimatesListThEstimate:       "Presupuesto",
	EstimatesListThClient:         "Cliente",
	EstimatesListThIssued:         "Emitido",
	EstimatesListThValidUntil:     "Válido Hasta",
	EstimatesListThStatus:         "Estado",
	EstimatesListThTotal:          "Total",
	EstimatesListThActions:        "Acciones",
	EstimatesListSend:             "Enviar",
	EstimatesListEdit:             "Editar",
	EstimatesListResend:           "Reenviar",
	EstimatesListView:             "Ver",
	EstimatesListConvert:          "Convertir",
	EstimatesListDeleteBtn:        "Eliminar",
	EstimatesListUntitled:         "Sin título",
	EstimatesListNoClient:         "Sin cliente",
	EstimatesListCreatedLabel:     "Creado",
	EstimatesListDeleteTitle:      "Eliminar Borrador de Presupuesto",
	EstimatesListDeleteBody:       "Esta acción no se puede deshacer. El presupuesto y todos sus artículos serán eliminados permanentemente.",
	EstimatesListDeleteConfirm:    "Escribe el número de presupuesto para confirmar:",
	EstimatesListDeletePlaceholder: "Escribe el número de presupuesto aquí",
	EstimatesListDeleteCancel:     "Cancelar",
	EstimatesListDeletePermanent:  "Eliminar Permanentemente",
	EstimatesListAcceptedTitle:    "Este presupuesto ha sido aceptado",
	EstimatesListAcceptedBody:     "El cliente ya aprobó este presupuesto. ¿Qué deseas hacer?",
	EstimatesListConvertToInv:     "Convertir a Factura",
	EstimatesListConvertSub:       "Recomendado — crea una factura a partir de este presupuesto aceptado",
	EstimatesListEditAnyway:       "Editar de Todas Formas",
	EstimatesListEditAnywayWarn:   "El estado volverá a borrador — el cliente deberá volver a aceptar",
	EstimatesListAcceptedCancel:   "Cancelar",

	LoginTitle:           "Bienvenido",
	LoginSubtitle:        "Inicia sesión en tu cuenta",
	LoginLimitBanner:     "Ya usaste tus facturas gratuitas. Inicia sesión o crea una cuenta para continuar — es gratis.",
	LoginEmail:           "Correo Electrónico",
	LoginPassword:        "Contraseña",
	LoginSubmit:          "Iniciar Sesión",
	LoginForgot:          "¿Olvidaste tu contraseña?",
	LoginOr:              "o",
	LoginGoogle:          "Continuar con Google",
	LoginGoogleSoon:      "Próximamente",
	LoginNoAccount:       "¿No tienes una cuenta?",
	LoginCreateOne:       "Créala aquí",

	RegisterTitle:        "Crea tu cuenta",
	RegisterSubtitle:     "Genera facturas. Rastrea pagos. Controla tus datos.",
	RegisterEmail:        "Correo Electrónico",
	RegisterPassword:     "Contraseña",
	RegisterPasswordHint: "Mínimo 8 caracteres",
	RegisterConfirm:      "Confirmar Contraseña",
	RegisterSubmit:       "Crear Cuenta",
	RegisterOr:           "o",
	RegisterGoogle:       "Continuar con Google",
	RegisterGoogleSoon:   "Próximamente",
	RegisterHaveAccount:  "¿Ya tienes una cuenta?",
	RegisterSignIn:       "Inicia sesión",

}

// Get returns the translation set for the given language code.
// Falls back to English for any unknown code.
func Get(lang string) T {
	if lang == "es" {
		return es
	}
	return en
}
