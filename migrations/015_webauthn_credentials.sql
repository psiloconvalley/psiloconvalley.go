-- Migration: 015_webauthn_credentials.sql
-- Applied: 2025-06-19
-- Rollback: DROP TABLE webauthn_credentials;
-- Risk: low — additive only, no existing tables modified

CREATE TABLE webauthn_credentials (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id   BYTEA NOT NULL UNIQUE,
    public_key      BYTEA NOT NULL,
    sign_count      BIGINT NOT NULL DEFAULT 0,
    device_name     TEXT NOT NULL DEFAULT 'Passkey',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at    TIMESTAMPTZ
);

CREATE INDEX idx_webauthn_credentials_user_id
    ON webauthn_credentials(user_id);
