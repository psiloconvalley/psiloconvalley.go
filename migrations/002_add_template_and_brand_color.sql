-- 002_add_template_and_brand_color.sql
--
-- Adds per-invoice template selection and brand color customization.
-- Both columns have defaults that match the current appearance exactly,
-- so existing invoices are visually unchanged after this migration.
--
-- template_id: selects which HTML template renders the invoice.
--   Values: 'classic', 'minimal', 'bold'
--   Default: 'classic' (current layout)
--
-- brand_color: hex accent color applied via CSS variable.
--   Format: #RRGGBB
--   Default: #0d1422 (current deep slate)

ALTER TABLE invoices
  ADD COLUMN template_id VARCHAR(20) NOT NULL DEFAULT 'classic',
  ADD COLUMN brand_color VARCHAR(7)  NOT NULL DEFAULT '#0d1422';
