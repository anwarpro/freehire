-- name: ListTeamBotSessions :many
SELECT id,user_id,preset,label,cv_id,job_id,created_at,updated_at FROM assistant_sessions
WHERE user_id=$1 AND preset='team_jobs' ORDER BY updated_at DESC,id DESC LIMIT 51 OFFSET $2;

-- name: ListTeamBotCourseJobs :many
-- Only automatic, non-private requirement evidence enters the bot. Human review data stays in team review.
SELECT j.public_slug,j.title,j.company,j.source,j.url,j.posted_at,j.last_seen_at,j.closed_at,
 m.suggested_tracks,m.computed_at,m.mapping_version,
 EXISTS(SELECT 1 FROM curriculum_outbox o WHERE o.job_id=j.id)::boolean AS update_pending
FROM curriculum_matches m JOIN jobs j ON j.id=m.job_id
WHERE NOT j.is_private AND m.mapping_version=sqlc.arg(mapping_version)
 AND (sqlc.arg(track)::text='' OR m.suggested_tracks @> ARRAY[sqlc.arg(track)::text])
 AND (sqlc.arg(source)::text='' OR j.source=sqlc.arg(source)::text)
 AND (sqlc.arg(state)::text='all' OR (sqlc.arg(state)::text='active' AND j.closed_at IS NULL) OR (sqlc.arg(state)::text='closed' AND j.closed_at IS NOT NULL))
ORDER BY m.job_id DESC LIMIT 11 OFFSET sqlc.arg(page_offset);
