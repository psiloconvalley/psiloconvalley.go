-- 003_add_stripe_connect_id_to_users.sql
-- Stores the Stripe Account ID for users who connect their Stripe accounts.
-- Format: acct_...

ALTER TABLE users
  ADD COLUMN stripe_connect_id VARCHAR(255);
