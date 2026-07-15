// internal/handlers/recurring.go
package handlers

import (
	"log/slog"
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
		slog.Error("recurring list failed", "err", err)
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
		slog.Error("recurring pause failed", "err", err)
		http.Error(w, "Failed to pause schedule", http.StatusInternalServerError)
		return
	}

	cancelled, err := h.App.SchedulerRepo.CancelJobsForRecurringSchedule(r.Context(), id)
	if err != nil {
		slog.Warn("recurring pause cancel jobs warning", "err", err)
	} else if cancelled > 0 {
		slog.Info("recurring jobs cancelled on pause", "count", cancelled, "schedule_id", id)
	}

	slog.Info("recurring schedule paused", "user_id", user.ID, "schedule_id", id)
	http.Redirect(w, r, "/recurring", http.StatusSeeOther)
}







// RecurringResume sets a schedule back to active and queues the next job.
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
		slog.Error("recurring resume failed", "err", err)
		http.Error(w, "Failed to resume schedule", http.StatusInternalServerError)
		return
	}

	sched, err := h.App.SchedulerRepo.GetRecurringScheduleByID(r.Context(), id)
	if err != nil {
		slog.Error("recurring resume fetch failed", "err", err)
		http.Error(w, "Failed to load schedule after resume", http.StatusInternalServerError)
		return
	}

	// Clear any stale pending jobs before scheduling a fresh one
	cancelled, err := h.App.SchedulerRepo.CancelJobsForRecurringSchedule(r.Context(), id)
	if err != nil {
		slog.Warn("recurring resume cancel stale jobs warning", "err", err)
	} else if cancelled > 0 {
		slog.Info("recurring stale jobs cleared", "count", cancelled, "schedule_id", id)
	}

	payload := map[string]any{"schedule_id": id}
	jobID, err := h.App.SchedulerRepo.CreateJob(r.Context(), "generate_recurring_invoice", payload, sched.NextRunAt)
	if err != nil {
		slog.Error("recurring resume next job failed", "err", err)
		http.Error(w, "Failed to schedule next recurring run", http.StatusInternalServerError)
		return
	}

	slog.Info("recurring schedule resumed", "user_id", user.ID, "schedule_id", id, "job_id", jobID, "next_run", sched.NextRunAt.Format("2006-01-02 15:04"))

	http.Redirect(w, r, "/recurring", http.StatusSeeOther)
}


// RecurringDelete removes a recurring schedule.
// RecurringDelete removes a recurring schedule and cancels pending jobs.
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

	cancelled, err := h.App.SchedulerRepo.CancelJobsForRecurringSchedule(r.Context(), id)
	if err != nil {
		slog.Warn("recurring delete cancel jobs warning", "err", err)
	} else if cancelled > 0 {
		slog.Info("recurring jobs cancelled before delete", "count", cancelled, "schedule_id", id)
	}

	if err := h.App.SchedulerRepo.DeleteRecurringScheduleByUser(r.Context(), id, user.ID); err != nil {
		slog.Error("recurring delete failed", "err", err)
		http.Error(w, "Failed to delete schedule", http.StatusInternalServerError)
		return
	}

	slog.Info("recurring schedule deleted", "user_id", user.ID, "schedule_id", id)
	http.Redirect(w, r, "/recurring", http.StatusSeeOther)
}
