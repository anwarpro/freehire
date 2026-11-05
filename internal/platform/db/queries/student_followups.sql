-- name: CreateStudentFollowup :one
INSERT INTO team_student_followups(job_id,student_ref,student_name,profile_url,reason,created_by,updated_by)
VALUES ($1,$2,$3,$4,$5,$6,$6) RETURNING *;

-- name: GetStudentFollowup :one
SELECT * FROM team_student_followups WHERE id=$1 AND job_id=$2 FOR UPDATE;

-- name: UpdateStudentFollowup :one
UPDATE team_student_followups SET student_name=$2,profile_url=$3,reason=$4,status=$5,notes=$6,follow_up=$7,
 updated_by=$8,updated_at=now(),version=version+1 WHERE id=$1 RETURNING *;

-- name: InsertStudentFollowupEvent :exec
INSERT INTO team_student_followup_events(followup_id,actor_id,action,student_name,profile_url,reason,status,notes,follow_up,version,channel,contacted_at,contact_note,request_id)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14);

-- name: GetStudentContactRequest :one
SELECT * FROM team_student_followup_events WHERE followup_id=$1 AND request_id=$2;

-- name: HasStudentContact :one
SELECT EXISTS(SELECT 1 FROM team_student_followup_events WHERE followup_id=$1 AND action='contacted')::boolean;

-- name: ListStudentFollowups :many
SELECT f.*,j.public_slug,j.title,j.company,j.url AS job_url,j.closed_at,
 COALESCE(m.name,u.email,'Unassigned')::text AS assignee,
 COALESCE(f.follow_up < sqlc.arg(today)::date AND f.status NOT IN ('offer','closed'),false)::boolean AS overdue,
 (SELECT count(*) FROM team_student_followup_events e WHERE e.followup_id=f.id AND e.action='contacted')::bigint AS contact_count
FROM team_student_followups f JOIN jobs j ON j.id=f.job_id
LEFT JOIN job_reviews r ON r.job_id=j.id
LEFT JOIN users u ON u.id=r.assignee_id LEFT JOIN team_members m ON m.user_id=u.id
WHERE NOT j.is_private
 AND (sqlc.arg(is_admin)::boolean OR r.assignee_id=sqlc.arg(actor_id)::bigint)
 AND (sqlc.arg(followup_id)::bigint=0 OR f.id=sqlc.arg(followup_id)::bigint)
 AND (sqlc.arg(slug)::text='' OR j.public_slug=sqlc.arg(slug)::text)
 AND (sqlc.arg(status)::text='' OR f.status=sqlc.arg(status)::text)
 AND (sqlc.arg(student_query)::text='' OR strpos(lower(f.student_ref),lower(sqlc.arg(student_query)::text))>0 OR strpos(lower(f.student_name),lower(sqlc.arg(student_query)::text))>0)
 AND (NOT sqlc.arg(overdue_only)::boolean OR (f.follow_up < sqlc.arg(today)::date AND f.status NOT IN ('offer','closed')))
ORDER BY f.updated_at DESC,f.id DESC LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: ListStudentFollowupEvents :many
SELECT e.*,COALESCE(m.name,u.email,'Deleted account')::text AS actor
FROM team_student_followup_events e JOIN team_student_followups f ON f.id=e.followup_id
JOIN jobs j ON j.id=f.job_id LEFT JOIN job_reviews r ON r.job_id=j.id
LEFT JOIN users u ON u.id=e.actor_id LEFT JOIN team_members m ON m.user_id=u.id
WHERE e.followup_id=sqlc.arg(followup_id) AND NOT j.is_private AND j.public_slug=sqlc.arg(slug)
 AND (sqlc.arg(is_admin)::boolean OR r.assignee_id=sqlc.arg(actor_id)::bigint)
ORDER BY e.version DESC LIMIT 100;
