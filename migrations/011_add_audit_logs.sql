-- migrations/011_add_audit_logs.sql
-- Audit logging foundation for HIPAA compliance.
-- Every action that touches financial or personal data is recorded here.
-- This table is append-only — rows are never updated or deleted.
--
-- action:      what happened       (invoice.created, auth.login, payment.received)
-- entity_type: what was affected   (invoice, estimate, user, payment)
-- entity_id:   which record        (the invoice ID, user ID, etc.)
-- ip_address:  who did it          (for security review and anomaly detection)
-- metadata:    structured context  (old status, new status, amount, etc.)

CREATE TABLE IF NOT EXISTS audit_logs (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action      TEXT        NOT NULL,
    entity_type TEXT        NOT NULL,
    entity_id   BIGINT,
    ip_address  TEXT,
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fast lookup by user — "show me everything this user did"
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id
    ON audit_logs(user_id);

-- Fast lookup by entity — "show me everything that happened to invoice #42"
CREATE INDEX IF NOT EXISTS idx_audit_logs_entity
    ON audit_logs(entity_type, entity_id);

-- Fast lookup by time — "show me all actions in the last 24 hours"
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at
    ON audit_logs(created_at DESC);

-- Fast lookup by action — "show me all failed logins"
CREATE INDEX IF NOT EXISTS idx_audit_logs_action
    ON audit_logs(action);

