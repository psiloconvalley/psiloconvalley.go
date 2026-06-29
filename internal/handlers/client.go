package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/catalog"
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
		slog.Error("client list query failed", "user_id", user.ID, "err", err)
	}

	count, _ := h.App.ClientRepo.CountByUserID(r.Context(), user.ID)

	h.App.Render(w, r, "clients_list.tmpl", map[string]any{
		"Clients":      clients,
		"ClientCount":  count,
		"AtLimit": !isPro(user.Plan) && count >= clientLimitFor(user.Plan),
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
		http.Redirect(w, r, "/pricing?reason=client-limit", http.StatusSeeOther)
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
		http.Redirect(w, r, "/pricing?reason=client-limit", http.StatusSeeOther)
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
		State:             catalog.NormalizeState(r.FormValue("state")),
		Zip:               strings.TrimSpace(r.FormValue("zip")),
		Country:           catalog.NormalizeCountry(r.FormValue("country")),
		Phone:             catalog.FormatPhone(r.FormValue("phone")),
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
		slog.Error("client create failed", "user_id", user.ID, "err", err)
		h.App.Render(w, r, "client_form.tmpl", map[string]any{
			"Mode":   "create",
			"Error":  "Could not save client",
			"Client": c,
		})
		return
	}

	slog.Info("client created", "client_id", newID, "name", c.Name, "biz_profile_id", c.BusinessProfileID)
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
		State:   catalog.NormalizeState(r.FormValue("state")),
		Zip:     strings.TrimSpace(r.FormValue("zip")),
		Country: catalog.NormalizeCountry(r.FormValue("country")),
		Phone:   catalog.FormatPhone(r.FormValue("phone")),
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
		slog.Error("client update failed", "user_id", user.ID, "err", err)
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
		slog.Error("client delete failed", "user_id", user.ID, "err", err)
		http.Error(w, "Could not delete client", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/clients", http.StatusSeeOther)
}
