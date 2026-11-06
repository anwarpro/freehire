-- migrate: no-transaction
-- squawk-ignore prefer-robust-stmts -- Revalidating an already valid CHECK is safe on retry.
ALTER TABLE assistant_sessions VALIDATE CONSTRAINT assistant_sessions_preset_check;
-- Build indexes without blocking catalogue matching or conversation writes.
CREATE INDEX CONCURRENTLY IF NOT EXISTS assistant_sessions_team_jobs_idx ON assistant_sessions(user_id,updated_at DESC,id DESC) WHERE preset='team_jobs';
CREATE INDEX CONCURRENTLY IF NOT EXISTS curriculum_matches_suggestions_idx ON curriculum_matches USING gin(suggested_tracks);
