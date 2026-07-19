package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"psiloconvalley/internal/app"
	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/repo"
	"github.com/resend/resend-go/v2"

)

type Handlers struct {
	App *app.App
}

func NewHandlers(a *app.App) *Handlers {
	return &Handlers{App: a}
}

func (h *Handlers) Index(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)

	// Anon users get the marketing page — no DB call needed
	if user == nil {
		h.App.Render(w, r, "home.tmpl", map[string]any{
			"Meta": app.DefaultMeta(
				"PSILOCONVALLEY — Invoices & Estimates in Minutes",
				"Professional invoicing for independent contractors. English & Spanish. Free to start.",
				"Send estimates, get approval, convert to invoice, and get paid. Free to start.",
				h.App.BaseURL,
			),
		})
		return
	}

	// Logged-in users get real business stats
	stats, err := h.App.InvRepo.GetDashboardStats(r.Context(), user.ID)
	if err != nil {
		slog.Error("home stats query failed", "user_id", user.ID, "err", err)
		stats = &repo.DashboardStats{}
	}

	h.App.Render(w, r, "home.tmpl", map[string]any{
		"Stats":  stats,
		"IsPro":  catalog.IsPaid(user.Plan),
		"Meta": app.DefaultMeta(
			"PSILOCONVALLEY — Invoices & Estimates in Minutes",
			"Professional invoicing for independent contractors. English & Spanish. Free to start.",
			"Send estimates, get approval, convert to invoice, and get paid. Free to start.",
			h.App.BaseURL,
		),
	})
}
func (h *Handlers) Research(w http.ResponseWriter, r *http.Request) {
	h.App.Render(w, r, "research.tmpl", map[string]any{
		"Meta": app.DefaultMeta(
			"Roadmap & Changelog | PSILOCONVALLEY",
			"What we are building next at psiloconvalley. Public roadmap, changelog, and shipped features for independent contractors.",
			"Public roadmap and changelog for psiloconvalley invoicing platform.",
			h.App.BaseURL+"/research",
		),
	})
}
func (h *Handlers) SecurityPage(w http.ResponseWriter, r *http.Request) {
	h.App.Render(w, r, "security.tmpl", map[string]any{
		"Meta": app.DefaultMeta(
			"Security | PSILOCONVALLEY",
			"How psiloconvalley protects your business data, client information, and payments. Argon2id, HMAC sessions, CSRF protection, and audit logs.",
			"Enterprise-grade security for your invoicing data. Argon2id, HMAC sessions, CSRF, audit logs.",
			h.App.BaseURL+"/security",
		),
	})
}
func (h *Handlers) ToolsHub(w http.ResponseWriter, r *http.Request) {
	h.App.Render(w, r, "tools_hub.tmpl", map[string]any{
		"Meta": app.DefaultMeta(
			"How It Works | PSILOCONVALLEY",
			"Send quotes, create invoices, accept card payments, and manage your clients. Professional tools for independent businesses. Free to start.",
			"Professional invoicing and quoting tools for independent businesses. Free to start.",
			h.App.BaseURL+"/tools",
		),
	})
}
func (h *Handlers) EnterprisePage(w http.ResponseWriter, r *http.Request) {
	h.App.Render(w, r, "enterprise.tmpl", map[string]any{
		"Meta": app.DefaultMeta(
			"Enterprise | PSILOCONVALLEY",
			"Secure, audit-logged invoicing for agencies, contractors, and professional services. Built on an enterprise security foundation.",
			"Audit logs, HMAC sessions, CSRF protection, and Argon2id password security. Professional invoicing you can trust.",
			h.App.BaseURL+"/enterprise",
		),
	})
}
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "ok")
}
func (h *Handlers) Feedback(w http.ResponseWriter, r *http.Request) {
	report := r.FormValue("report")

	user := auth.GetUser(r)
	fromEmail := "Anonymous"
	if user != nil {
		fromEmail = user.Email
	}

	// Log to Railway (backup)
	slog.Info("feature request received", "from", fromEmail, "content", report)

	// Email to yourself via Resend
	go func() {
		if h.App.Mailer == nil {
			return
		}

		client := resend.NewClient("")
		if key := h.App.Mailer.APIKey(); key != "" {
			client = resend.NewClient(key)
		} else {
			slog.Warn("feature request email skipped, no API key")
			return
		}

		subject := fmt.Sprintf("[psiloconvalley Feature Request] from %s", fromEmail)
		body := fmt.Sprintf(
			"<h2>Feature Request</h2>"+
				"<p><strong>From:</strong> %s</p>"+
				"<p><strong>Time:</strong> %s</p>"+
				"<hr>"+
				"<p>%s</p>",
			fromEmail,
			time.Now().Format("2006-01-02 15:04:05 MST"),
			report,
		)

		_, err := client.Emails.Send(&resend.SendEmailRequest{
			From:    "noreply@psiloconvalley.com",
			To:      []string{"psiloconvalleypsiloconvalley@gmail.com"},
			Subject: subject,
			Html:    body,
		})
		if err != nil {
			slog.Error("feature request email failed", "err", err)
		} else {
			slog.Info("feature request email sent")
		}
	}()

	http.Redirect(w, r, "/tools?transmitted=true", http.StatusSeeOther)
}
func (h *Handlers) NotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	h.App.Render(w, r, "404.tmpl", nil)
}
