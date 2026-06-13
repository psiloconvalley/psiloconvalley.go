package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"psiloconvalley/internal/app"
	"psiloconvalley/internal/auth"
	"github.com/resend/resend-go/v2"

)

type Handlers struct {
	App *app.App
}

func NewHandlers(a *app.App) *Handlers {
	return &Handlers{App: a}
}

// ---------- Basic pages ----------

func (h *Handlers) Index(w http.ResponseWriter, r *http.Request) {
    h.App.Render(w, r, "home.tmpl", nil)
}
func (h *Handlers) Research(w http.ResponseWriter, r *http.Request) {
	h.App.Render(w, r, "research.tmpl", nil)
}
func (h *Handlers) SecurityPage(w http.ResponseWriter, r *http.Request) {
	h.App.Render(w, r, "security.tmpl", nil)
}

func (h *Handlers) ToolsHub(w http.ResponseWriter, r *http.Request) {
	h.App.Render(w, r, "tools_hub.tmpl", nil)
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
	log.Printf(
		"\n[ANOMALY REPORT] %s\nFROM: %s\nCONTENT: %s\n-------------------",
		time.Now().Format("2006-01-02 15:04:05"),
		fromEmail,
		report,
	)

	// Email to yourself via Resend
	go func() {
		if h.App.Mailer == nil {
			return
		}

		client := resend.NewClient("")
		if key := h.App.Mailer.APIKey(); key != "" {
			client = resend.NewClient(key)
		} else {
			log.Println("[feedback] no Resend API key, skipping email")
			return
		}

		subject := fmt.Sprintf("[PsiloConValley Feedback] from %s", fromEmail)
		body := fmt.Sprintf(
			"<h2>Feedback Report</h2>"+
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
			log.Printf("[feedback] email error: %v", err)
		} else {
			log.Printf("[feedback] emailed to psiloconvalleypsiloconvalley@gmail.com")
		}
	}()

	http.Redirect(w, r, "/tools?transmitted=true", http.StatusSeeOther)
}
func (h *Handlers) NotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	h.App.Render(w, r, "404.tmpl", nil)
}
