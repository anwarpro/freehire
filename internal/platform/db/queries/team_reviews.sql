-- name: GetTeamAccess :one
SELECT u.id, u.email, COALESCE(m.name, u.email)::text AS name,
       CASE WHEN u.role IN ('moderator','admin') THEN 'admin'
            WHEN m.active THEN 'reviewer' ELSE 'none' END::text AS team_role
FROM users u LEFT JOIN team_members m ON m.user_id=u.id
WHERE u.id=$1;

-- name: LockTeamUsers :many
-- Every team write locks users in the same order before touching a job/member.
-- Deactivation and assignment therefore cannot race a stale authorization read.
SELECT id FROM users WHERE id=ANY($1::bigint[]) ORDER BY id FOR UPDATE;

-- name: ListTeamMembers :many
SELECT u.id, u.email, COALESCE(m.name,u.email)::text AS name,
       CASE WHEN u.role IN ('moderator','admin') THEN 'admin' ELSE 'reviewer' END::text AS team_role,
       CASE WHEN u.role IN ('moderator','admin') THEN true ELSE m.active END::boolean AS active,
       COALESCE(m.version,0)::bigint AS version,
       (SELECT count(*) FROM job_reviews r WHERE r.assignee_id=u.id)::bigint AS assignment_count
FROM users u LEFT JOIN team_members m ON m.user_id=u.id
WHERE u.role IN ('moderator','admin') OR m.user_id IS NOT NULL
ORDER BY name,u.id LIMIT 501;

-- name: GetTeamMember :one
SELECT m.*, u.email, u.role FROM team_members m JOIN users u ON u.id=m.user_id
WHERE m.user_id=$1;

-- name: CreateTeamMember :one
INSERT INTO team_members(user_id,name,created_by) VALUES ($1,$2,$3) RETURNING *;

-- name: UpdateTeamMember :one
UPDATE team_members SET name=$2,active=$3,version=version+1,updated_at=now()
WHERE user_id=$1 AND version=$4 RETURNING *;

-- name: InsertTeamMemberEvent :exec
INSERT INTO team_member_events(member_id,actor_id,action) VALUES ($1,$2,$3);

-- name: ListTeamMemberEvents :many
SELECT e.id,e.action,e.created_at,COALESCE(m.name,u.email,'Deleted account')::text AS actor
FROM team_member_events e LEFT JOIN users u ON u.id=e.actor_id
LEFT JOIN team_members m ON m.user_id=u.id
WHERE e.member_id=$1 ORDER BY e.id DESC LIMIT 100;

-- name: GetTeamJob :one
SELECT id FROM jobs WHERE public_slug=$1 AND NOT is_private FOR UPDATE;

-- name: GetTeamReviewRecord :one
SELECT * FROM job_reviews WHERE job_id=$1;

-- name: GetTeamJobReview :one
SELECT j.id AS job_id,j.public_slug,j.title,j.company,j.closed_at,
       r.assignee_id,COALESCE(m.name,u.email,'Unassigned')::text AS assignee,
       CASE WHEN u.role IN ('moderator','admin') THEN true ELSE COALESCE(m.active,false) END::boolean AS assignee_active,
       COALESCE(r.status,'pending')::text AS status,COALESCE(r.notes,'')::text AS notes,
       r.follow_up,COALESCE(r.version,0)::bigint AS version,r.updated_at,
       COALESCE(r.follow_up < sqlc.arg(today)::date AND r.status IN ('pending','in_review'),false)::boolean AS overdue
FROM jobs j LEFT JOIN job_reviews r ON r.job_id=j.id
LEFT JOIN users u ON u.id=r.assignee_id LEFT JOIN team_members m ON m.user_id=u.id
WHERE j.public_slug=sqlc.arg(slug) AND NOT j.is_private
  AND (sqlc.arg(is_admin)::boolean OR r.assignee_id=sqlc.arg(actor_id)::bigint);

-- name: ListTeamReviews :many
SELECT j.id AS job_id,j.public_slug,j.title,j.company,j.closed_at,
       r.assignee_id,COALESCE(m.name,u.email,'Unassigned')::text AS assignee,
       CASE WHEN u.role IN ('moderator','admin') THEN true ELSE COALESCE(m.active,false) END::boolean AS assignee_active,
       r.status,r.notes,r.follow_up,r.version,r.updated_at,
       COALESCE(r.follow_up < sqlc.arg(today)::date AND r.status IN ('pending','in_review'),false)::boolean AS overdue
FROM job_reviews r JOIN jobs j ON j.id=r.job_id
LEFT JOIN users u ON u.id=r.assignee_id LEFT JOIN team_members m ON m.user_id=u.id
WHERE NOT j.is_private
  AND (sqlc.arg(is_admin)::boolean OR r.assignee_id=sqlc.arg(actor_id)::bigint)
  AND (sqlc.arg(scope)::text='all' OR (sqlc.arg(scope)::text='mine' AND r.assignee_id=sqlc.arg(actor_id)::bigint) OR (sqlc.arg(scope)::text='unassigned' AND r.assignee_id IS NULL))
  AND (sqlc.arg(status)::text='' OR r.status=sqlc.arg(status)::text)
  AND (NOT sqlc.arg(overdue_only)::boolean OR (r.follow_up < sqlc.arg(today)::date AND r.status IN ('pending','in_review')))
ORDER BY r.updated_at DESC,r.job_id DESC LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: SaveTeamReview :one
INSERT INTO job_reviews(job_id,assignee_id,status,notes,follow_up,updated_by)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT(job_id) DO UPDATE SET assignee_id=EXCLUDED.assignee_id,status=EXCLUDED.status,
notes=EXCLUDED.notes,follow_up=EXCLUDED.follow_up,updated_by=EXCLUDED.updated_by,
version=job_reviews.version+1,updated_at=now() RETURNING *;

-- name: InsertTeamReviewEvent :exec
INSERT INTO job_review_events(job_id,actor_id,assignee_id,action,status,notes,follow_up,version)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8);

-- name: ListTeamReviewEvents :many
SELECT e.id,e.action,e.status,e.notes,e.follow_up,e.version,e.created_at,
       COALESCE(am.name,au.email,'Deleted account')::text AS actor,
       COALESCE(sm.name,su.email,'Unassigned')::text AS assignee
FROM job_review_events e JOIN jobs j ON j.id=e.job_id
LEFT JOIN users au ON au.id=e.actor_id LEFT JOIN team_members am ON am.user_id=au.id
LEFT JOIN users su ON su.id=e.assignee_id LEFT JOIN team_members sm ON sm.user_id=su.id
WHERE j.id=sqlc.arg(job_id) AND NOT j.is_private
 AND (sqlc.arg(is_admin)::boolean OR EXISTS (SELECT 1 FROM job_reviews r WHERE r.job_id=j.id AND r.assignee_id=sqlc.arg(actor_id)::bigint))
 ORDER BY e.id DESC LIMIT 100;
