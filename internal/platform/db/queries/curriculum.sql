-- name: GetCurriculumConfig :one
SELECT * FROM curriculum_config WHERE singleton;

-- name: LockCurriculumConfig :one
SELECT * FROM curriculum_config WHERE singleton FOR SHARE;

-- name: UpdateCurriculumConfig :one
UPDATE curriculum_config SET payload=$1,updated_by=$2,version=version+1,updated_at=now()
WHERE singleton AND version=$3 RETURNING *;

-- name: InsertCurriculumConfigEvent :exec
INSERT INTO curriculum_config_events(version,payload,actor_id) VALUES ($1,$2,$3);

-- name: ListCurriculumConfigEvents :many
SELECT e.version,e.created_at,e.payload,CASE WHEN e.version=1 AND e.actor_id IS NULL THEN 'Initial defaults' ELSE COALESCE(m.name,u.email,'Deleted account') END::text AS actor
FROM curriculum_config_events e LEFT JOIN users u ON u.id=e.actor_id
LEFT JOIN team_members m ON m.user_id=u.id ORDER BY e.version DESC LIMIT 30;

-- name: ResetCurriculumScan :exec
UPDATE curriculum_scan_state SET mapping_version=$1,cursor_id=0,upper_id=0,complete=false,processed=0,last_error=''
WHERE singleton;

-- name: GetCurriculumScan :one
SELECT * FROM curriculum_scan_state WHERE singleton;

-- name: InitializeCurriculumScan :exec
UPDATE curriculum_scan_state SET upper_id=COALESCE((SELECT id FROM jobs ORDER BY id DESC LIMIT 1),0)
WHERE singleton AND mapping_version=$1 AND upper_id=0 AND NOT complete;

-- name: AdvanceCurriculumScan :exec
UPDATE curriculum_scan_state SET cursor_id=$2,complete=$3
WHERE singleton AND mapping_version=$1;

-- name: RecordCurriculumRun :exec
UPDATE curriculum_scan_state SET processed=processed+$1,last_run_at=now(),last_error=$2 WHERE singleton;

-- name: ListCurriculumDirty :many
SELECT job_id FROM curriculum_outbox ORDER BY job_id LIMIT $1;

-- name: ListCurriculumSeed :many
SELECT id FROM jobs WHERE id > sqlc.arg(cursor_id) AND id <= sqlc.arg(upper_id)
AND NOT is_private AND closed_at IS NULL ORDER BY id LIMIT sqlc.arg(page_limit);

-- name: DeleteCurriculumDirty :exec
DELETE FROM curriculum_outbox WHERE job_id=$1;

-- name: LockCurriculumJob :one
-- Worker reads privacy only to remove projections for pasted JDs. Team callers
-- authorize through GetTeamJobReview before reaching this row lock.
SELECT id,title,description,is_private FROM jobs WHERE id=$1 FOR UPDATE;

-- name: DeleteCurriculumMatch :exec
DELETE FROM curriculum_matches WHERE job_id=$1;

-- name: GetCurriculumMatch :one
SELECT * FROM curriculum_matches WHERE job_id=$1;

-- name: UpsertCurriculumMatch :one
INSERT INTO curriculum_matches(job_id,mapping_version,input_hash,evidence,suggested_tracks)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT(job_id) DO UPDATE SET mapping_version=EXCLUDED.mapping_version,
input_hash=EXCLUDED.input_hash,evidence=EXCLUDED.evidence,suggested_tracks=EXCLUDED.suggested_tracks,
computed_at=now() RETURNING *;

-- name: ReviewCurriculumMatch :one
UPDATE curriculum_matches SET reviewed_tracks=$2,reviewed_by=$3,reviewed_at=now(),
reviewed_hash=input_hash,reviewed_mapping_version=mapping_version,review_notes=$4,review_version=review_version+1
WHERE job_id=$1 RETURNING *;

-- name: InsertCurriculumReviewEvent :exec
INSERT INTO curriculum_review_events(job_id,actor_id,tracks,notes,input_hash,mapping_version,version)
VALUES ($1,$2,$3,$4,$5,$6,$7);

-- name: ListCurriculumReviewEvents :many
SELECT e.id,e.tracks,e.notes,e.input_hash,e.mapping_version,e.version,e.created_at,
COALESCE(m.name,u.email,'Deleted account')::text AS actor
FROM curriculum_review_events e JOIN jobs j ON j.id=e.job_id
LEFT JOIN users u ON u.id=e.actor_id LEFT JOIN team_members m ON m.user_id=u.id
WHERE e.job_id=sqlc.arg(job_id) AND NOT j.is_private
AND (sqlc.arg(is_admin)::boolean OR EXISTS(SELECT 1 FROM job_reviews r WHERE r.job_id=j.id AND r.assignee_id=sqlc.arg(actor_id)))
ORDER BY e.id DESC LIMIT 100;

-- name: ListCurriculumJobs :many
SELECT j.id AS job_id,j.public_slug,j.title,j.company,j.location,j.source,j.closed_at,
m.suggested_tracks,m.reviewed_tracks,m.effective_tracks,m.review_version,m.computed_at,
(m.reviewed_tracks IS NOT NULL)::boolean AS reviewed,
(m.reviewed_tracks IS NOT NULL AND (m.reviewed_hash<>m.input_hash OR m.reviewed_mapping_version<>m.mapping_version))::boolean AS review_stale,
EXISTS(SELECT 1 FROM curriculum_outbox o WHERE o.job_id=j.id)::boolean AS update_pending
FROM curriculum_matches m JOIN jobs j ON j.id=m.job_id
LEFT JOIN job_reviews r ON r.job_id=j.id
WHERE NOT j.is_private AND m.mapping_version=sqlc.arg(mapping_version)
AND (sqlc.arg(is_admin)::boolean OR r.assignee_id=sqlc.arg(actor_id))
AND (sqlc.arg(track)::text='' OR m.effective_tracks @> ARRAY[sqlc.arg(track)::text])
AND (sqlc.arg(source)::text='' OR j.source=sqlc.arg(source)::text)
AND (sqlc.arg(job_state)::text='all' OR (sqlc.arg(job_state)::text='active' AND j.closed_at IS NULL) OR (sqlc.arg(job_state)::text='closed' AND j.closed_at IS NOT NULL))
AND (sqlc.arg(review_state)::text='' OR (sqlc.arg(review_state)::text='unreviewed' AND m.reviewed_tracks IS NULL)
 OR (sqlc.arg(review_state)::text='reviewed' AND m.reviewed_tracks IS NOT NULL)
 OR (sqlc.arg(review_state)::text='needs_review' AND m.reviewed_tracks IS NOT NULL AND
 (m.reviewed_hash<>m.input_hash OR m.reviewed_mapping_version<>m.mapping_version OR EXISTS(SELECT 1 FROM curriculum_outbox o WHERE o.job_id=j.id))))
ORDER BY m.job_id DESC LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);
