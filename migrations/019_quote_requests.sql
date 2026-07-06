CREATE TABLE IF NOT EXISTS quote_requests (
    id                  BIGSERIAL PRIMARY KEY,
    business_profile_id BIGINT NOT NULL REFERENCES business_profiles(id),
    client_name         TEXT NOT NULL,
    client_phone        TEXT NOT NULL DEFAULT '',
    description         TEXT NOT NULL DEFAULT '',
    status              TEXT NOT NULL DEFAULT 'new',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_quote_requests_business_profile_id
    ON quote_requests (business_profile_id);
