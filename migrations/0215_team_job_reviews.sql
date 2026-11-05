-- Apply before deploying the team workspace. Existing moderator/admin accounts
-- manage the team; reviewers are an independent, revocable membership.
CREATE TABLE team_members (
    user_id bigint PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    active boolean NOT NULL DEFAULT true,
    version bigint NOT NULL DEFAULT 1,
    created_by bigint REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX team_members_created_by_idx ON team_members(created_by);

CREATE TABLE team_member_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    member_id bigint REFERENCES users(id) ON DELETE SET NULL,
    actor_id bigint REFERENCES users(id) ON DELETE SET NULL,
    action text NOT NULL CHECK (action IN ('created', 'added', 'updated', 'activated', 'deactivated')),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX team_member_events_member_idx ON team_member_events(member_id, id DESC);
CREATE INDEX team_member_events_actor_idx ON team_member_events(actor_id);

CREATE TABLE job_reviews (
    job_id bigint PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,
    assignee_id bigint REFERENCES users(id) ON DELETE SET NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'in_review', 'reviewed', 'not_relevant')),
    notes text NOT NULL DEFAULT '' CHECK (char_length(notes) <= 5000),
    follow_up date,
    version bigint NOT NULL DEFAULT 1,
    updated_by bigint REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX job_reviews_assignee_idx ON job_reviews(assignee_id, updated_at DESC, job_id DESC);
CREATE INDEX job_reviews_updated_by_idx ON job_reviews(updated_by);
CREATE INDEX job_reviews_queue_idx ON job_reviews(updated_at DESC, job_id DESC);

CREATE TABLE job_review_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    job_id bigint NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    actor_id bigint REFERENCES users(id) ON DELETE SET NULL,
    assignee_id bigint REFERENCES users(id) ON DELETE SET NULL,
    action text NOT NULL CHECK (action IN ('assigned', 'reassigned', 'unassigned', 'updated')),
    status text NOT NULL CHECK (status IN ('pending', 'in_review', 'reviewed', 'not_relevant')),
    notes text NOT NULL CHECK (char_length(notes) <= 5000),
    follow_up date,
    version bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (job_id, version)
);
CREATE INDEX job_review_events_actor_idx ON job_review_events(actor_id);
CREATE INDEX job_review_events_assignee_idx ON job_review_events(assignee_id);
