ALTER TABLE business_profiles
    ADD COLUMN IF NOT EXISTS slug TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_business_profiles_slug
    ON business_profiles (slug)
    WHERE slug != '';
