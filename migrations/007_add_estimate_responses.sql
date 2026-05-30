-- 007_add_estimate_responses.sql
-- Stores client responses to estimates (accept, decline, suggest).

CREATE TABLE estimate_responses (
    id          BIGSERIAL PRIMARY KEY,
    estimate_id BIGINT NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    action      VARCHAR(20) NOT NULL CHECK (action IN ('accepted', 'declined', 'suggestion')),
    message     TEXT,
    client_name VARCHAR(255),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_estimate_responses_estimate_id ON estimate_responses (estimate_id);
