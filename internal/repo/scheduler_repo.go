package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// ScheduledJob matches the scheduled_jobs table
type ScheduledJob struct {
	ID            int64
	JobType       string
	Payload       json.RawMessage
	Status        string
	RunAt         time.Time
	StartedAt     *time.Time
	CompletedAt   *time.Time
	FailedAt      *time.Time
	FailureReason *string
	Attempts      int
	MaxAttempts   int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type SchedulerRepo struct {
	db *sql.DB
}

func NewSchedulerRepo(db *sql.DB) *SchedulerRepo {
	return &SchedulerRepo{db: db}
}

// FetchNextJobs picks up pending jobs that are due.
// This uses the critical FOR UPDATE SKIP LOCKED pattern to scale.
func (r *SchedulerRepo) FetchNextJobs(ctx context.Context, limit int) ([]ScheduledJob, error) {
	const q = `
		SELECT id, job_type, payload, status, run_at, started_at, 
		       completed_at, failed_at, failure_reason, attempts, max_attempts,
		       created_at, updated_at
		FROM scheduled_jobs
		WHERE status = 'pending' AND run_at <= NOW()
		ORDER BY run_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED`

	rows, err := r.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []ScheduledJob
	for rows.Next() {
		var j ScheduledJob
		var started, completed, failed sql.NullTime
		var reason sql.NullString

		err := rows.Scan(
			&j.ID, &j.JobType, &j.Payload, &j.Status, &j.RunAt,
			&started, &completed, &failed, &reason,
			&j.Attempts, &j.MaxAttempts, &j.CreatedAt, &j.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if started.Valid   { j.StartedAt = &started.Time }
		if completed.Valid { j.CompletedAt = &completed.Time }
		if failed.Valid    { j.FailedAt = &failed.Time }
		if reason.Valid    { j.FailureReason = &reason.String }

		jobs = append(jobs, j)
	}
	return jobs, nil
}

// MarkRunning moves a job from pending to running
func (r *SchedulerRepo) MarkRunning(ctx context.Context, id int64) error {
	const q = `UPDATE scheduled_jobs SET status = 'running', started_at = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, q, id)
	return err
}

// MarkComplete finishes a job successfully
func (r *SchedulerRepo) MarkComplete(ctx context.Context, id int64) error {
	const q = `UPDATE scheduled_jobs SET status = 'completed', completed_at = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, q, id)
	return err
}

// MarkFailed handles job failure and increments attempts
func (r *SchedulerRepo) MarkFailed(ctx context.Context, id int64, reason string, nextRun *time.Time) error {
	if nextRun != nil {
		// Reschedule for later (backoff)
		const q = `
			UPDATE scheduled_jobs 
			SET status = 'pending', 
			    attempts = attempts + 1, 
			    run_at = $2, 
			    failure_reason = $3,
			    updated_at = NOW() 
			WHERE id = $1`
		_, err := r.db.ExecContext(ctx, q, id, *nextRun, reason)
		return err
	}

	// Permanent failure
	const q = `
		UPDATE scheduled_jobs 
		SET status = 'failed', 
		    failed_at = NOW(), 
		    failure_reason = $2,
		    updated_at = NOW() 
		WHERE id = $1`
	_, err := r.db.ExecContext(ctx, q, id, reason)
	return err
}

// LogEvent writes to the job_logs table
func (r *SchedulerRepo) LogEvent(ctx context.Context, jobID int64, event, detail string) error {
	const q = `INSERT INTO job_logs (job_id, event, detail) VALUES ($1, $2, $3)`
	_, err := r.db.ExecContext(ctx, q, jobID, event, detail)
	return err
}

// CreateJob allows other parts of the app to queue work
func (r *SchedulerRepo) CreateJob(ctx context.Context, jobType string, payload any, runAt time.Time) (int64, error) {
	marshaled, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	const q = `
		INSERT INTO scheduled_jobs (job_type, payload, run_at, status)
		VALUES ($1, $2, $3, 'pending')
		RETURNING id`
	
	var id int64
	err = r.db.QueryRowContext(ctx, q, jobType, marshaled, runAt).Scan(&id)
	return id, err

}


// CancelJobsForInvoice deletes all pending reminder jobs for a given invoice.
// Called when an invoice is marked paid or voided.
func (r *SchedulerRepo) CancelJobsForInvoice(ctx context.Context, invoiceID int64) (int64, error) {
	const q = `
		DELETE FROM scheduled_jobs
		WHERE status = 'pending'
		AND job_type = 'send_reminder'
		AND payload->>'invoice_id' = $1::text`

	result, err := r.db.ExecContext(ctx, q, invoiceID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
