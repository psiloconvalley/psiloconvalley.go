package app

import (
	"bytes"
	"database/sql"
	"fmt"
	"strings"
	"path/filepath"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/justinas/nosurf"
	"github.com/stripe/stripe-go/v81"
	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/i18n"
	"psiloconvalley/internal/logo"
	"psiloconvalley/internal/receipt"
	"psiloconvalley/internal/mailer"
	"psiloconvalley/internal/repo"
	"psiloconvalley/internal/scheduler"
	schedulerhandlers "psiloconvalley/internal/scheduler/handlers"
	"psiloconvalley/internal/util"
	"psiloconvalley/internal/service"
)

type App struct {
	db            *sql.DB
	Templates     *template.Template
	InvRepo       repo.InvoiceStore
	InvService     *service.InvoiceService
	ClientRepo    *repo.ClientRepo
	BizRepo       *repo.BusinessRepo
	UserRepo      *repo.UserRepo
	Mailer        *mailer.Mailer
	BaseURL       string
	StripePrice   string
	StripeGrowthPrice string
	SchedulerRepo *repo.SchedulerRepo
	Scheduler     *scheduler.Scheduler
	LogoStore     logo.Store
	ReceiptStore  receipt.Store
	EstRespRepo   *repo.EstimateResponseRepo
	ExpenseRepo   *repo.ExpenseRepo
	AuditRepo  *repo.AuditRepo
	UsageRepo  *repo.UsageRepo
	PasskeyRepo *repo.PasskeyRepo
	AddressRepo *repo.AddressRepo
}
// DB returns the underlying database connection for direct queries.
func (a *App) DB() *sql.DB { return a.db }

func NewApp(db *sql.DB) *App {
	auth.InitSessionSecret()
	
	funcs := template.FuncMap{
		"money":        util.Money,
		"formatCents":  util.FormatCentsForInput,
		"bpsToPercent": util.BpsToPercent,
		"field":        field,
		"mul":          func(a, b int) int { return a * b },
		"hasPrefix":    strings.HasPrefix,
		"hasSuffix":    strings.HasSuffix,
	}
	// Two-pass parse: root templates + partials subdirectory.
	// Add additional ParseGlob calls here for new subdirectories.
	t, err := template.New("").Funcs(funcs).ParseGlob("templates/*.tmpl")
	if err != nil {
		log.Fatal("template parse error:", err)
	}

	partialMatches, err := filepath.Glob("templates/partials/*.tmpl")
	if err != nil {
		log.Fatal("template glob error (partials):", err)
	}
	if len(partialMatches) > 0 {
		t, err = t.ParseGlob("templates/partials/*.tmpl")
		if err != nil {
			log.Fatal("template parse error (partials):", err)
		}
	}

	ogMatches, err := filepath.Glob("templates/og/*.tmpl")
	if err != nil {
		log.Fatal("template glob error (og):", err)
	}
	if len(ogMatches) > 0 {
		t, err = t.ParseGlob("templates/og/*.tmpl")
		if err != nil {
			log.Fatal("template parse error (og):", err)
		}
	}

	baseURL := os.Getenv("APP_BASE_URL")
	if baseURL == "" {
		baseURL = "https://psiloconvalley.com"
	}

	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	stripeGrowthPrice := os.Getenv("STRIPE_GROWTH_PRICE_ID")
	stripePrice := os.Getenv("STRIPE_PRICE_ID")

	schedRepo := repo.NewSchedulerRepo(db)
	sched := scheduler.New(schedRepo, 60*time.Second)

	// ── Register Job Handlers ─────────────────────────────────────────
	// Add new job types here. The engine never changes.
	invRepo := repo.NewInvoiceRepo(db)
	
	invService := service.NewInvoiceService(
    		invRepo,
    		repo.NewUserRepo(db),
    		schedRepo,
)
	sched.Register("send_reminder", schedulerhandlers.NewReminderHandler(
		invRepo,
		mailer.New(),
		baseURL,
	))
	sched.Register("generate_recurring_invoice", schedulerhandlers.NewRecurringHandler(
		invRepo,
		schedRepo,
		repo.NewUserRepo(db),
		mailer.New(),
		baseURL,
	))


	return &App{
		AddressRepo:       repo.NewAddressRepo(db),
		db:                db,
		Templates:         t,
		InvRepo:           invRepo,
		InvService:        invService,
		ClientRepo:        repo.NewClientRepo(db),
		BizRepo:           repo.NewBusinessRepo(db),
		UserRepo:          repo.NewUserRepo(db),
		Mailer:            mailer.New(),
		BaseURL:           baseURL,
		StripePrice:       stripePrice,
		StripeGrowthPrice: stripeGrowthPrice,
		SchedulerRepo:     schedRepo,
		Scheduler:         sched,
		LogoStore:         newLogoStore(baseURL),
		ReceiptStore:      newReceiptStore(baseURL),
		EstRespRepo:       repo.NewEstimateResponseRepo(db),
		ExpenseRepo:       repo.NewExpenseRepo(db),
		AuditRepo: 	   repo.NewAuditRepo(db),
		UsageRepo:  	   repo.NewUsageRepo(db),
		PasskeyRepo: repo.NewPasskeyRepo(db),
	}
}
	
// newLogoStore selects the logo backend based on LOGO_STORAGE env var.
// LOGO_STORAGE=supabase → SupabaseStore (production)
func newLogoStore(baseURL string) logo.Store {
	if os.Getenv("LOGO_STORAGE") == "supabase" {
		store, err := logo.NewSupabaseStore()
		if err != nil {
			log.Fatalf("supabase store init: %v", err)
		}
		return store
	}
	return logo.NewLocalStore("static/uploads/logos", baseURL)
}

// newReceiptStore selects the receipt backend based on LOGO_STORAGE env var.
// Uses the same env var as logos — both go to Supabase in production.
func newReceiptStore(baseURL string) receipt.Store {
	if os.Getenv("LOGO_STORAGE") == "supabase" {
		store, err := receipt.NewSupabaseStore()
		if err != nil {
			log.Fatalf("supabase receipt store init: %v", err)
		}
		return store
	}
	return receipt.NewLocalStore("static/uploads/receipts", baseURL)
}
func field(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case *string:
		if val == nil {
			return ""
		}
		return *val
	case int64:
		if val == 0 {
			return ""
		}
		return fmt.Sprintf("%d", val)
	case float64:
		if val == 0.0 {
			return ""
		}
		return fmt.Sprintf("%.2f", val)
	case time.Time:
		if val.IsZero() {
			return ""
		}
		return val.Format("2006-01-02")
	default:
		return fmt.Sprintf("%v", val)
	}
}

func (a *App) Render(w http.ResponseWriter, r *http.Request, name string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}

	user := auth.GetUser(r)
	data["User"] = user
	data["GoogleEnabled"] = auth.GoogleOAuthEnabled()
	data["csrfField"] = template.HTML(
		fmt.Sprintf(`<input type="hidden" name="csrf_token" value="%s">`, nosurf.Token(r)),
	)

	// ── Inject translations based on user language preference ─────────
lang := "en"
if user != nil && user.Language != "" {
    lang = user.Language
} else if strings.Contains(r.Header.Get("Accept-Language"), "es") {
    lang = "es"
}
	data["T"] = i18n.Get(lang)
	data["Lang"] = lang
	var buf bytes.Buffer
	if err := a.Templates.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("template error [%s]: %v", name, err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}
