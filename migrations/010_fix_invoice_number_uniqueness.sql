-- Migration 010: Fix invoice number uniqueness
-- OLD: global uniqueness on invoice_number (two different users cannot share INV-0001)
-- NEW: per-user uniqueness on (user_id, invoice_number)
-- Anonymous invoices (user_id IS NULL) are not constrained.

ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_invoice_number_key;
DROP INDEX IF EXISTS invoices_invoice_number_key;
DROP INDEX IF EXISTS idx_invoices_invoice_number;

CREATE UNIQUE INDEX idx_invoices_user_invoice_number
ON invoices (user_id, invoice_number)
WHERE user_id IS NOT NULL AND invoice_number IS NOT NULL;




