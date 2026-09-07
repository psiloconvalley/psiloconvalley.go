-- migrations/024_clear_legacy_invoice_company_snapshots.sql
-- Clears all legacy hardcoded snapshot fields on invoices linked to a business profile.
-- This guarantees all existing invoices use their live business profile identity (San Mateo).

UPDATE invoices
SET 
    company_name = '',
    company_email = '',
    company_address = '',
    company_city = '',
    company_zip = '',
    company_state = '',
    company_country = ''
WHERE business_profile_id IS NOT NULL;
