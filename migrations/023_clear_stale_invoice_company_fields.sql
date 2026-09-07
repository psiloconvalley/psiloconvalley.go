-- migrations/023_clear_stale_invoice_company_fields.sql
-- Clears out hardcoded company fields from invoices where they match the owner's business profile defaults.
-- This safely restores the live profile fallback (e.g. San Mateo city updates) on existing invoices.

UPDATE invoices i
SET 
    company_name = CASE WHEN i.company_name = bp.name THEN '' ELSE i.company_name END,
    company_email = CASE WHEN i.company_email = bp.email THEN '' ELSE i.company_email END,
    company_address = CASE WHEN i.company_address = bp.address THEN '' ELSE i.company_address END,
    company_city = CASE WHEN i.company_city = bp.city THEN '' ELSE i.company_city END,
    company_zip = CASE WHEN i.company_zip = bp.zip THEN '' ELSE i.company_zip END,
    company_state = CASE WHEN i.company_state = bp.state THEN '' ELSE i.company_state END,
    company_country = CASE WHEN i.company_country = bp.country THEN '' ELSE i.company_country END
FROM business_profiles bp
WHERE bp.id = i.business_profile_id;
