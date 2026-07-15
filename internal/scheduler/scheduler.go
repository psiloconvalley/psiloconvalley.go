// internal/scheduler/scheduler.go
package scheduler

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"psiloconvalley/internal/repo"
)

// =====================================================================
// Scheduler Engine
//
// Runs as a goroutine inside the main process.
// Every tick it fetches pending jobs that are due, executes them,
// and logs the result.
//
// Adding a new job type:
//   1. Define a payload struct
//   2. Write a HandlerFunc
//   3. Register it in NewScheduler with Register()
//
// The engine never changes when you add new job types.
// =====================================================================

// HandlerFunc is the signature every job handler must implement.
// It receives the raw JSON payload and returns an error or nil.
type HandlerFunc func(ctx context.Context, payload json.RawMessage) error

// Scheduler is the engine.
type Scheduler struct {
	repo     *repo.SchedulerRepo
	handlers map[string]HandlerFunc
	interval time.Duration
}

// New creates a Scheduler with the given tick interval.
func New(r *repo.SchedulerRepo, interval time.Duration) *Scheduler {
	return &Scheduler{
		repo:     r,
		handlers: make(map[string]HandlerFunc),
		interval: interval,
	}
}

// Register adds a job type and its handler to the engine.
func (s *Scheduler) Register(jobType string, fn HandlerFunc) {
	s.handlers[jobType] = fn
}

// Start runs the scheduler loop. Call this in a goroutine.
// It blocks until ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	slog.Info("scheduler started", "interval", s.interval)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler shutting down")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick fetches and executes all pending due jobs
func (s *Scheduler) tick(ctx context.Context) {
	jobs, err := s.repo.FetchNextJobs(ctx, 10)
	if err != nil {
		slog.Error("scheduler fetch failed", "err", err)
		return
	}

	for _, job := range jobs {
		s.execute(ctx, job)
	}
}

// execute runs a single job
func (s *Scheduler) execute(ctx context.Context, job repo.ScheduledJob) {
	slog.Info("scheduler executing job", "job_id", job.ID, "job_type", job.JobType)

	// Mark as running
	if err := s.repo.MarkRunning(ctx, job.ID); err != nil {
		slog.Error("scheduler mark running failed", "job_id", job.ID, "err", err)
		return
	}
	_ = s.repo.LogEvent(ctx, job.ID, "started", "")

	// Find handler
	handler, ok := s.handlers[job.JobType]
	if !ok {
		reason := "no handler registered for job type: " + job.JobType
		slog.Warn("scheduler job skipped", "job_id", job.ID, "reason", reason)
		_ = s.repo.MarkFailed(ctx, job.ID, reason, nil)
		_ = s.repo.LogEvent(ctx, job.ID, "failed", reason)
		return
	}

	// Execute handler
	err := handler(ctx, job.Payload)
	if err != nil {
		slog.Error("scheduler job failed", "job_id", job.ID, "attempt", job.Attempts+1, "err", err)
		_ = s.repo.LogEvent(ctx, job.ID, "error", err.Error())

		// Decide: retry or permanent fail
		newAttempts := job.Attempts + 1
		if newAttempts >= job.MaxAttempts {
			_ = s.repo.MarkFailed(ctx, job.ID, err.Error(), nil)
			_ = s.repo.LogEvent(ctx, job.ID, "failed", "max attempts reached")
			slog.Error("scheduler job permanently failed", "job_id", job.ID)
			return
		}

		// Exponential backoff: 5min, 30min, 2hr
		backoff := backoffDuration(newAttempts)
		next := time.Now().Add(backoff)
		_ = s.repo.MarkFailed(ctx, job.ID, err.Error(), &next)
		_ = s.repo.LogEvent(ctx, job.ID, "retrying", backoff.String())
		slog.Warn("scheduler job retry scheduled", "job_id", job.ID, "backoff", backoff)
		return
	}

	// Success
	_ = s.repo.MarkComplete(ctx, job.ID)
	_ = s.repo.LogEvent(ctx, job.ID, "completed", "")
	slog.Info("scheduler job completed", "job_id", job.ID)
}

// backoffDuration returns how long to wait before retrying
// based on the attempt number.
func backoffDuration(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 5 * time.Minute
	case 2:
		return 30 * time.Minute
	default:
		return 2 * time.Hour
	}
}
