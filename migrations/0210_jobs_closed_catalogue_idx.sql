-- migrate: no-transaction
-- Keyset pagination over retained public canonical jobs, without counting the archive.
-- If interrupted, check pg_index.indisvalid and drop an invalid index concurrently
-- before retrying; IF NOT EXISTS alone cannot repair an invalid build.
CREATE INDEX CONCURRENTLY IF NOT EXISTS jobs_closed_catalogue_idx
    ON jobs (closed_at DESC, public_slug DESC)
    WHERE closed_at IS NOT NULL AND duplicate_of IS NULL AND NOT is_private;
