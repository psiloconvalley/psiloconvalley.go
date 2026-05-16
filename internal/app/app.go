package app

import (
	"bytes"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/justinas/nosurf"
	"github.com/stripe/stripe-go/v81"
	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/mailer"
	"psiloconvalley/internal/repo"
	"psiloconvalley/internal/util"
)

type App struct {
	Templates   *template.Template
	InvRepo     *repo.InvoiceRepo
	ClientRepo  *repo.ClientRepo
	BizRepo     *repo.BusinessRepo
	UserRepo    *repo.UserRepo
	Mailer      *mailer.Mailer
	BaseURL     string
	StripePrice string
}

func NewApp(db *sql.DB) *App {
	funcs := template.FuncMap{
		"money":        util.Money,
		"formatCents":  util.FormatCentsForInput,
		"bpsToPercent": util.BpsToPercent,
		"field":        field,
	}

	// Using ParseGlob during migration (easier than embed for now)
	t, err := template.New("").Funcs(funcs).ParseGlob("templates/*.tmpl")
	if err != nil {
		log.Fatal("template parse error:", err)
	}

	baseURL := os.Getenv("APP_BASE_URL")
	if baseURL == "" {
		baseURL = "https://psiloconvalley.com"
	}

	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	stripePrice := os.Getenv("STRIPE_PRICE_ID")
	return &App{
		Templates:   t,
		InvRepo:     repo.NewInvoiceRepo(db),
		ClientRepo:  repo.NewClientRepo(db),
		BizRepo:     repo.NewBusinessRepo(db),
		UserRepo:    repo.NewUserRepo(db),
		Mailer:      mailer.New(),
		BaseURL:     baseURL,
		StripePrice: stripePrice,
	}
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

	data["User"] = auth.GetUser(r)
	data["GoogleEnabled"] = auth.GoogleOAuthEnabled()
	data["csrfField"] = template.HTML(
		fmt.Sprintf(`<input type="hidden" name="csrf_token" value="%s">`, nosurf.Token(r)),
	)

	var buf bytes.Buffer
	if err := a.Templates.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("template error [%s]: %v", name, err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}
