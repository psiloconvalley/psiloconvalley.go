package handlers

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/repo"
)

func (h *Handlers) ClientsList(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	clients, err := h.App.ClientRepo.ListByUserID(r.Context(), user.ID)
	if err != nil {
		log.Printf("list clients error: %v", err)
	}

	count, _ := h.App.ClientRepo.CountByUserID(r.Context(), user.ID)

	h.App.Render(w, r, "clients_list.tmpl", map[string]any{
		"Clients":      clients,
		"ClientCount":  count,
		"AtLimit":      user.Plan != "pro" && count >= 3,
		"CanAddClient": h.canAddClient(r),
		"saved":        r.URL.Query().Get("saved") == "true",
	})
}

func (h *Handlers) ClientNewGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if !h.canAddClient(r) {
		http.Redirect(w, r, "/clients?limit=true", http.StatusSeeOther)
		return
	}

	h.App.Render(w, r, "client_form.tmpl", map[string]any{"Mode": "create"})
}

func (h *Handlers) ClientNewPost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if !h.canAddClient(r) {
		http.Redirect(w, r, "/clients?limit=true", http.StatusSeeOther)
		return
	}

	profile, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID)
	if err != nil {
		h.App.Render(w, r, "client_form.tmpl", map[string]any{
			"Mode":  "create",
			"Error": "Please set up your business profile first.",
		})
		return
	}

	c := &repo.Client{
		BusinessProfileID: profile.ID,
		Name:              strings.TrimSpace(r.FormValue("name")),
		Email:             strings.TrimSpace(r.FormValue("email")),
		Address:           strings.TrimSpace(r.FormValue("address")),
		City:              strings.TrimSpace(r.FormValue("city")),
		State:             strings.TrimSpace(r.FormValue("state")),
		Zip:               strings.TrimSpace(r.FormValue("zip")),
		Country:           strings.TrimSpace(r.FormValue("country")),
		Phone:             strings.TrimSpace(r.FormValue("phone")),
		Notes:             strings.TrimSpace(r.FormValue("notes")),
	}

	if c.Name == "" {
		h.App.Render(w, r, "client_form.tmpl", map[string]any{
			"Mode":   "create",
			"Error":  "Client name is required",
			"Client": c,
		})
		return
	}

	newID, err := h.App.ClientRepo.Create(r.Context(), c)
	if err != nil {
		log.Printf("create client error: %v", err)
		h.App.Render(w, r, "client_form.tmpl", map[string]any{
			"Mode":   "create",
			"Error":  "Could not save client",
			"Client": c,
		})
		return
	}

	log.Printf("CLIENT CREATED: id=%d name=%s biz_profile=%d", newID, c.Name, c.BusinessProfileID)
	http.Redirect(w, r, "/clients?saved=true", http.StatusSeeOther)
}

func (h *Handlers) ClientEditGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	client, err := h.App.ClientRepo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	h.App.Render(w, r, "client_form.tmpl", map[string]any{
		"Mode":   "edit",
		"Client": client,
	})
}

func (h *Handlers) ClientEditPost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	c := &repo.Client{
		ID:      id,
		Name:    strings.TrimSpace(r.FormValue("name")),
		Email:   strings.TrimSpace(r.FormValue("email")),
		Address: strings.TrimSpace(r.FormValue("address")),
		City:    strings.TrimSpace(r.FormValue("city")),
		State:   strings.TrimSpace(r.FormValue("state")),
		Zip:     strings.TrimSpace(r.FormValue("zip")),
		Country: strings.TrimSpace(r.FormValue("country")),
		Phone:   strings.TrimSpace(r.FormValue("phone")),
		Notes:   strings.TrimSpace(r.FormValue("notes")),
	}

	if c.Name == "" {
		h.App.Render(w, r, "client_form.tmpl", map[string]any{
			"Mode":   "edit",
			"Error":  "Client name is required",
			"Client": c,
		})
		return
	}

	if err := h.App.ClientRepo.Update(r.Context(), c, user.ID); err != nil {
		log.Printf("update client error: %v", err)
		h.App.Render(w, r, "client_form.tmpl", map[string]any{
			"Mode":   "edit",
			"Error":  "Could not update client",
			"Client": c,
		})
		return
	}

	http.Redirect(w, r, "/clients?saved=true", http.StatusSeeOther)
}

func (h *Handlers) ClientDelete(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	if err := h.App.ClientRepo.Delete(r.Context(), id, user.ID); err != nil {
		log.Printf("delete client error: %v", err)
		http.Error(w, "Could not delete client", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/clients", http.StatusSeeOther)
}
