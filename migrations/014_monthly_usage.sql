-- Migration: 014_monthly_usage.sql
-- Applied: 2025-06-17
-- Rollback: DROP TABLE monthly_usage;
-- Risk: low — additive only

CREATE TABLE IF NOT EXISTS monthly_usage (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    year_month  TEXT NOT NULL,
    invoices    INT NOT NULL DEFAULT 0,
    sends       INT NOT NULL DEFAULT 0,
    estimates   INT NOT NULL DEFAULT 0,
    reports     INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE(user_id, year_month)
);

CREATE INDEX IF NOT EXISTS idx_monthly_usage_user_month ON monthly_usage(user_id, year_month);
