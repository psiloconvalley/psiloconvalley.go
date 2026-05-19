-- =====================================================================
-- Migration 001: Scheduler Engine Tables
-- Run this manually in Railway PostgreSQL console
-- Date: 2026-05-19
-- =====================================================================

-- ── Scheduled Jobs ────────────────────────────────────────────────────
-- The core job queue. Every scheduled action in the system is a row
-- in this table. job_type determines which handler executes it.
-- payload is JSONB so each job type can carry its own data structure.
-- FOR UPDATE SKIP LOCKED on fetch prevents double-execution when
-- multiple app instances are running.
-- ─────────────────────────────────────────────────────────────────────
CREATE TABLE scheduled_jobs (
    id              BIGSERIAL PRIMARY KEY,
    job_type        TEXT NOT NULL,
    payload         JSONB NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','running','completed','failed')),
    run_at          TIMESTAMPTZ NOT NULL,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    failed_at       TIMESTAMPTZ,
    failure_reason  TEXT,
    attempts        INT NOT NULL DEFAULT 0,
    max_attempts    INT NOT NULL DEFAULT 3,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fast lookup: find pending jobs that are due
CREATE INDEX idx_scheduled_jobs_pending_run_at
    ON scheduled_jobs(run_at)
    WHERE status = 'pending';

-- Fast lookup: find jobs by status for monitoring
CREATE INDEX idx_scheduled_jobs_status
    ON scheduled_jobs(status);

-- ── Job Logs ──────────────────────────────────────────────────────────
-- Audit trail for every job execution event.
-- Never delete these. They are your debugging history.
-- ─────────────────────────────────────────────────────────────────────
CREATE TABLE job_logs (
    id          BIGSERIAL PRIMARY KEY,
    job_id      BIGINT NOT NULL REFERENCES scheduled_jobs(id) ON DELETE CASCADE,
    event       TEXT NOT NULL,
    detail      TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_job_logs_job_id ON job_logs(job_id);

-- ── Invoice Reminder Settings ─────────────────────────────────────────
-- One row per invoice that has reminders enabled.
-- remind_before: days before due date to send first reminder
-- remind_after:  array of days after due date to send follow-ups
-- ─────────────────────────────────────────────────────────────────────
CREATE TABLE invoice_reminder_settings (
    id              BIGSERIAL PRIMARY KEY,
    invoice_id      BIGINT NOT NULL REFERENCES invoices(id) ON DELETE CASCADE UNIQUE,
    enabled         BOOLEAN NOT NULL DEFAULT false,
    remind_before   INT NOT NULL DEFAULT 3,
    remind_after    INT[] NOT NULL DEFAULT '{3,7,14}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_invoice_reminder_settings_invoice_id
    ON invoice_reminder_settings(invoice_id);

-- ── Recurring Schedules ───────────────────────────────────────────────
-- One row per recurring invoice setup.
-- template_invoice_id points to the invoice used as the template.
-- frequency drives when the next job is scheduled.
-- send_automatically: true = auto-send, false = create draft only
-- ─────────────────────────────────────────────────────────────────────
CREATE TABLE recurring_schedules (
    id                    BIGSERIAL PRIMARY KEY,
    user_id               BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    template_invoice_id   BIGINT NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    frequency             TEXT NOT NULL
                              CHECK (frequency IN ('weekly','monthly','quarterly','yearly')),
    send_automatically    BOOLEAN NOT NULL DEFAULT false,
    active                BOOLEAN NOT NULL DEFAULT true,
    next_run_at           TIMESTAMPTZ NOT NULL,
    last_run_at           TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_recurring_schedules_user_id
    ON recurring_schedules(user_id);

CREATE INDEX idx_recurring_schedules_next_run_at
    ON recurring_schedules(next_run_at)
    WHERE active = true;
