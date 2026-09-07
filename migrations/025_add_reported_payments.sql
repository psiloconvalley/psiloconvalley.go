-- migrations/025_add_reported_payments.sql
-- Tracks when a client reports an offline payment (Zelle, check, cash) has been sent.
-- This allows automated operator notification and dashboard alerts.

ALTER TABLE invoices 
ADD COLUMN payment_reported_at TIMESTAMPTZ,
ADD COLUMN payment_reported_method TEXT DEFAULT '',
ADD COLUMN payment_reported_note TEXT DEFAULT '';
