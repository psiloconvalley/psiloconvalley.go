// internal/router/router.go
package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/handlers"
)

func NewRouter(h *handlers.Handlers) http.Handler {
	r := chi.NewRouter()

	r.Use(handlers.SecurityHeaders)
	r.Use(auth.LoadUser(h.App.UserRepo))

	r.Handle("/static/*", http.StripPrefix("/static/",
		http.FileServer(http.Dir("static"))))

	r.Get("/", h.Index)
	r.Get("/tools", h.ToolsHub)
	r.Get("/healthz", h.Health)
	r.Post("/feedback", h.Feedback)

	r.Get("/register", h.RegisterGet)
	r.Post("/register", h.RegisterPost)

	r.Get("/login", h.LoginGet)
	r.Post("/login", h.LoginPost)

	r.Post("/logout", h.Logout)

	r.Get("/auth/google", auth.GoogleLoginHandler)
	r.Get("/auth/google/callback", h.GoogleCallback)

	r.Get("/profile", h.ProfileGet)
	r.Post("/profile", h.ProfilePost)

	r.Get("/clients", h.ClientsList)
	r.Get("/clients/new", h.ClientNewGet)
	r.Post("/clients/new", h.ClientNewPost)
	r.Get("/research", h.Research)

	r.Route("/invoices", func(r chi.Router) {
		r.Get("/", h.InvoicesList)
		r.Get("/new", h.InvoiceNewGet)
		r.Post("/create", h.InvoiceCreatePost)
		r.Get("/{id}", h.InvoiceDetail)
		r.Get("/{id}/edit", h.InvoiceEditGet)
		r.Post("/{id}/edit", h.InvoiceUpdatePost)
		r.Post("/{id}/status", h.InvoiceStatusPost)
		r.Get("/{id}/pdf", h.InvoicePDFGet)
		r.Get("/{id}/send", h.InvoiceSendGet)
		r.Post("/{id}/send", h.InvoiceSendPost)
		// FIX H2: duplicate route was defined in the handler but never
		// registered. It is now reachable.
		r.Get("/{id}/duplicate", h.InvoiceDuplicateGet)
	})

	return r
}
