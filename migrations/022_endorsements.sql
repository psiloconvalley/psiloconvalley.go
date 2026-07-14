-- 022: Endorsements table for public business profiles.
-- Endorsements are requested by business owners for paid invoices.
-- Clients submit via token link — no login required.
-- Status: pending → submitted (live immediately) / declined (never public).
-- Business owner can delete any endorsement.

CREATE TABLE IF NOT EXISTS endorsements (
    id                  BIGSERIAL PRIMARY KEY,
    business_profile_id BIGINT NOT NULL REFERENCES business_profiles(id) ON DELETE CASCADE,
    invoice_id          BIGINT REFERENCES invoices(id) ON DELETE SET NULL,

    -- Client identity (filled on submission)
    endorser_name       VARCHAR(255) NOT NULL DEFAULT '',
    endorser_location   VARCHAR(255) NOT NULL DEFAULT '',

    -- The endorsement
    rating              INT NOT NULL DEFAULT 0 CHECK (rating >= 0 AND rating <= 5),
    body                TEXT NOT NULL DEFAULT '',

    -- Flow control
    token               VARCHAR(64) NOT NULL,
    status              VARCHAR(20) NOT NULL DEFAULT 'pending',

    -- Timestamps
    requested_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    submitted_at        TIMESTAMPTZ,

    CONSTRAINT endorsements_token_unique UNIQUE (token),
    CONSTRAINT endorsements_status_check CHECK (status IN ('pending', 'submitted', 'declined'))
);

CREATE INDEX IF NOT EXISTS idx_endorsements_business_profile
    ON endorsements(business_profile_id);
CREATE INDEX IF NOT EXISTS idx_endorsements_token
    ON endorsements(token);
CREATE INDEX IF NOT EXISTS idx_endorsements_status
    ON endorsements(business_profile_id, status);
