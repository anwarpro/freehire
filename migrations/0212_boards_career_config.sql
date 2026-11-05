-- Nullable configuration for curator-managed custom company career pages.
-- Existing ATS boards keep their existing adapter and require no backfill.
ALTER TABLE boards ADD COLUMN career_config jsonb;
