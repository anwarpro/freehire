-- migrate: no-transaction
-- The two-minute expiry sweep must visit only open jobs with stated deadlines.
-- A failed concurrent build may leave an invalid index: check indisvalid and drop
-- that index concurrently before retrying this migration.
CREATE INDEX CONCURRENTLY IF NOT EXISTS jobs_open_application_deadline_idx
    ON jobs (application_deadline)
    WHERE closed_at IS NULL AND application_deadline IS NOT NULL;
