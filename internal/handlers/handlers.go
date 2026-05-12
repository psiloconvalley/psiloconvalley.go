package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"psiloconvalley/internal/app"
	"psiloconvalley/internal/auth"
)

type Handlers struct {
	App *app.App
}

func NewHandlers(a *app.App) *Handlers {
	return &Handlers{App: a}
}

// ---------- Basic pages ----------

func (h *Handlers) Index(w http.ResponseWriter, r *http.Request) {
	h.App.Render(w, r, "index.tmpl", nil)
}
func (h *Handlers) Research(w http.ResponseWriter, r *http.Request) {
	h.App.Render(w, r, "research.tmpl", nil)
}

func (h *Handlers) ToolsHub(w http.ResponseWriter, r *http.Request) {
	h.App.Render(w, r, "tools_hub.tmpl", nil)
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "ok")
}

func (h *Handlers) InvoicePortal(w http.ResponseWriter, r *http.Request) {
	h.App.Render(w, r, "home.tmpl", nil)
}

func (h *Handlers) Feedback(w http.ResponseWriter, r *http.Request) {
	report := r.FormValue("report")

	user := auth.GetUser(r)
	email := "Anonymous"
	if user != nil {
		email = user.Email
	}

	log.Printf(
		"\n[ANOMALY REPORT] %s\nFROM: %s\nCONTENT: %s\n-------------------",
		time.Now().Format("2006-01-02 15:04:05"),
		email,
		report,
	)

	http.Redirect(w, r, "/tools?transmitted=true", http.StatusSeeOther)
}
