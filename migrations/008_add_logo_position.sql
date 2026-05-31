-- 008_add_logo_position.sql
-- Adds logo alignment control to invoices.
-- Default 'left' preserves all existing invoice layouts.

ALTER TABLE invoices
ADD COLUMN IF NOT EXISTS logo_position VARCHAR(10) NOT NULL DEFAULT 'left';
