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

// =====================================================================
// Recurring Schedules
// =====================================================================

type RecurringSchedule struct {
	ID                int64
	UserID            int64
	TemplateInvoiceID int64
	Frequency         string // weekly, monthly, quarterly, yearly
	SendAutomatically bool
	Active            bool
	NextRunAt         time.Time
	LastRunAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// CreateRecurringSchedule inserts a new recurring schedule and returns its ID.
func (r *SchedulerRepo) CreateRecurringSchedule(ctx context.Context, sched *RecurringSchedule) (int64, error) {
	const q = `
		INSERT INTO recurring_schedules 
			(user_id, template_invoice_id, frequency, send_automatically, active, next_run_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`

	var id int64
	err := r.db.QueryRowContext(ctx, q,
		sched.UserID,
		sched.TemplateInvoiceID,
		sched.Frequency,
		sched.SendAutomatically,
		sched.Active,
		sched.NextRunAt,
	).Scan(&id)
	return id, err
}

// GetRecurringScheduleByInvoice returns the recurring schedule for a template invoice, if any.
func (r *SchedulerRepo) GetRecurringScheduleByInvoice(ctx context.Context, invoiceID int64) (*RecurringSchedule, error) {
	const q = `
		SELECT id, user_id, template_invoice_id, frequency, send_automatically, 
		       active, next_run_at, last_run_at, created_at, updated_at
		FROM recurring_schedules
		WHERE template_invoice_id = $1`

	var s RecurringSchedule
	var lastRun sql.NullTime

	err := r.db.QueryRowContext(ctx, q, invoiceID).Scan(
		&s.ID, &s.UserID, &s.TemplateInvoiceID, &s.Frequency, &s.SendAutomatically,
		&s.Active, &s.NextRunAt, &lastRun, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if lastRun.Valid {
		s.LastRunAt = &lastRun.Time
	}
	return &s, nil
}

// UpdateRecurringSchedule updates an existing schedule (e.g., after generating an invoice).
func (r *SchedulerRepo) UpdateRecurringSchedule(ctx context.Context, sched *RecurringSchedule) error {
	const q = `
		UPDATE recurring_schedules
		SET frequency = $1, send_automatically = $2, active = $3, 
		    next_run_at = $4, last_run_at = $5, updated_at = NOW()
		WHERE id = $6`

	_, err := r.db.ExecContext(ctx, q,
		sched.Frequency,
		sched.SendAutomatically,
		sched.Active,
		sched.NextRunAt,
		sched.LastRunAt,
		sched.ID,
	)
	return err
}

// DeleteRecurringSchedule removes a recurring schedule.
func (r *SchedulerRepo) DeleteRecurringSchedule(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM recurring_schedules WHERE id = $1`, id)
	return err
}

// ListActiveRecurringSchedulesDue returns all active schedules where next_run_at <= now.
func (r *SchedulerRepo) ListActiveRecurringSchedulesDue(ctx context.Context) ([]RecurringSchedule, error) {
	const q = `
		SELECT id, user_id, template_invoice_id, frequency, send_automatically,
		       active, next_run_at, last_run_at, created_at, updated_at
		FROM recurring_schedules
		WHERE active = true AND next_run_at <= NOW()
		ORDER BY next_run_at ASC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []RecurringSchedule
	for rows.Next() {
		var s RecurringSchedule
		var lastRun sql.NullTime
		err := rows.Scan(
			&s.ID, &s.UserID, &s.TemplateInvoiceID, &s.Frequency, &s.SendAutomatically,
			&s.Active, &s.NextRunAt, &lastRun, &s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if lastRun.Valid {
			s.LastRunAt = &lastRun.Time
		}
		schedules = append(schedules, s)
	}
	return schedules, rows.Err()
}
// GetRecurringScheduleByID fetches a single recurring schedule by its ID.
func (r *SchedulerRepo) GetRecurringScheduleByID(ctx context.Context, id int64) (*RecurringSchedule, error) {
	const q = `
		SELECT id, user_id, template_invoice_id, frequency, send_automatically,
		       active, next_run_at, last_run_at, created_at, updated_at
		FROM recurring_schedules
		WHERE id = $1`

	var s RecurringSchedule
	var lastRun sql.NullTime

	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&s.ID, &s.UserID, &s.TemplateInvoiceID, &s.Frequency, &s.SendAutomatically,
		&s.Active, &s.NextRunAt, &lastRun, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if lastRun.Valid {
		s.LastRunAt = &lastRun.Time
	}
	return &s, nil
}
// RecurringScheduleView is a display-friendly version that includes invoice details.
type RecurringScheduleView struct {
	ID                int64
	TemplateInvoiceID int64
	InvoiceNumber     string
	ClientName        string
	Frequency         string
	SendAutomatically bool
	Active            bool
	NextRunAt         time.Time
	LastRunAt         *time.Time
}

// ListRecurringByUserID returns all recurring schedules for a user, joined with invoice details.
func (r *SchedulerRepo) ListRecurringByUserID(ctx context.Context, userID int64) ([]RecurringScheduleView, error) {
	const q = `
		SELECT rs.id, rs.template_invoice_id, i.invoice_number, i.client_name,
		       rs.frequency, rs.send_automatically, rs.active, rs.next_run_at, rs.last_run_at
		FROM recurring_schedules rs
		JOIN invoices i ON i.id = rs.template_invoice_id
		WHERE rs.user_id = $1
		ORDER BY rs.created_at DESC`

	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var views []RecurringScheduleView
	for rows.Next() {
		var v RecurringScheduleView
		var lastRun sql.NullTime
		err := rows.Scan(
			&v.ID, &v.TemplateInvoiceID, &v.InvoiceNumber, &v.ClientName,
			&v.Frequency, &v.SendAutomatically, &v.Active, &v.NextRunAt, &lastRun,
		)
		if err != nil {
			return nil, err
		}
		if lastRun.Valid {
			v.LastRunAt = &lastRun.Time
		}
		views = append(views, v)
	}
	return views, rows.Err()
}

// PauseRecurringSchedule sets active = false.
func (r *SchedulerRepo) PauseRecurringSchedule(ctx context.Context, id int64, userID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE recurring_schedules SET active = false, updated_at = NOW() WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	return err
}

// ResumeRecurringSchedule sets active = true.
func (r *SchedulerRepo) ResumeRecurringSchedule(ctx context.Context, id int64, userID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE recurring_schedules SET active = true, updated_at = NOW() WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	return err
}

// DeleteRecurringScheduleByUser deletes a schedule only if owned by the user.
func (r *SchedulerRepo) DeleteRecurringScheduleByUser(ctx context.Context, id int64, userID int64) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM recurring_schedules WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	return err
}
// CancelJobsForRecurringSchedule deletes all pending recurring-generation jobs
// for a given schedule ID.
func (r *SchedulerRepo) CancelJobsForRecurringSchedule(ctx context.Context, scheduleID int64) (int64, error) {
	const q = `
		DELETE FROM scheduled_jobs
		WHERE status = 'pending'
		AND job_type = 'generate_recurring_invoice'
		AND payload->>'schedule_id' = $1::text`

	result, err := r.db.ExecContext(ctx, q, scheduleID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
