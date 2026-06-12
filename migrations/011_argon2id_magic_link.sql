-- 011: Argon2id password hashing + magic link authentication
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_algo TEXT NOT NULL DEFAULT 'bcrypt';
ALTER TABLE users ADD COLUMN IF NOT EXISTS magic_token TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS magic_token_expires_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_users_magic_token ON users (magic_token) WHERE magic_token IS NOT NULL;
