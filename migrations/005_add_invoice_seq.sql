-- 005_add_invoice_seq.sql
-- Sequential invoice numbering per user.
-- Starts at 1. Never reused. Manual override still allowed.
ALTER TABLE users ADD COLUMN next_invoice_seq INTEGER NOT NULL DEFAULT 1;
