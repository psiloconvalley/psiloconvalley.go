-- Migration: 016_add_phone_to_business_profiles.sql
-- Adds optional phone field to business profiles.
-- Displays on invoices when present.
ALTER TABLE business_profiles ADD COLUMN IF NOT EXISTS phone TEXT NOT NULL DEFAULT '';
