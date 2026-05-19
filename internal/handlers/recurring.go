// internal/handlers/recurring.go
package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"psiloconvalley/internal/auth"
)

// RecurringList shows all recurring schedules for the logged-in user.
func (h *Handlers) RecurringList(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	schedules, err := h.App.SchedulerRepo.ListRecurringByUserID(r.Context(), user.ID)
	if err != nil {
		log.Printf("[recurring] list error: %v", err)
		http.Error(w, "Failed to load recurring schedules", http.StatusInternalServerError)
		return
	}

	h.App.Render(w, r, "recurring_list.tmpl", map[string]any{
		"Schedules": schedules,
		"User":      user,
	})
}

// RecurringPause sets a schedule to inactive.
func (h *Handlers) RecurringPause(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := h.App.SchedulerRepo.PauseRecurringSchedule(r.Context(), id, user.ID); err != nil {
		log.Printf("[recurring] pause error: %v", err)
		http.Error(w, "Failed to pause schedule", http.StatusInternalServerError)
		return
	}

	log.Printf("[recurring] user %d paused schedule %d", user.ID, id)
	http.Redirect(w, r, "/recurring", http.StatusSeeOther)
}

// RecurringResume sets a schedule back to active.
func (h *Handlers) RecurringResume(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := h.App.SchedulerRepo.ResumeRecurringSchedule(r.Context(), id, user.ID); err != nil {
		log.Printf("[recurring] resume error: %v", err)
		http.Error(w, "Failed to resume schedule", http.StatusInternalServerError)
		return
	}

	log.Printf("[recurring] user %d resumed schedule %d", user.ID, id)
	http.Redirect(w, r, "/recurring", http.StatusSeeOther)
}

// RecurringDelete removes a recurring schedule.
func (h *Handlers) RecurringDelete(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := h.App.SchedulerRepo.DeleteRecurringScheduleByUser(r.Context(), id, user.ID); err != nil {
		log.Printf("[recurring] delete error: %v", err)
		http.Error(w, "Failed to delete schedule", http.StatusInternalServerError)
		return
	}

	log.Printf("[recurring] user %d deleted schedule %d", user.ID, id)
	http.Redirect(w, r, "/recurring", http.StatusSeeOther)
}
