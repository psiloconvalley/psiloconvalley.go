-- 004_add_public_token_to_invoices.sql
-- Adds a public access token for sharing invoices with clients.
-- Used in email links so clients can view and pay invoices
-- without requiring login. Sequential IDs alone are not safe
-- for public access.

ALTER TABLE invoices
  ADD COLUMN public_token VARCHAR(64);

CREATE UNIQUE INDEX idx_invoices_public_token
  ON invoices (public_token)
  WHERE public_token IS NOT NULL;
