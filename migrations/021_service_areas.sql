-- 021: Add service areas to business profiles.
-- Free-text field for listing areas the business serves.
-- Displayed on public profile page, indexed by search engines.
ALTER TABLE business_profiles
ADD COLUMN IF NOT EXISTS service_areas TEXT NOT NULL DEFAULT '';
