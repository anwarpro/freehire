-- migrate: no-transaction
-- Closed jobs are absent from Meilisearch. Array filters need their own bounded
-- archive indexes rather than scanning every retained posting for a rare skill.
-- Check indisvalid after an interrupted build and drop an invalid index concurrently.
CREATE INDEX CONCURRENTLY IF NOT EXISTS jobs_closed_facets_idx
    ON jobs USING gin (skills, countries)
    WHERE closed_at IS NOT NULL AND duplicate_of IS NULL AND NOT is_private;
