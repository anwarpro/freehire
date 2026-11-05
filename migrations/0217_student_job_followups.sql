-- Apply before the app upgrade. References belong to a job, not a student directory.
CREATE TABLE team_student_followups (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 job_id bigint NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
 student_ref text NOT NULL CHECK (char_length(student_ref) BETWEEN 1 AND 300),
 student_name text NOT NULL DEFAULT '' CHECK (char_length(student_name)<=120),
 profile_url text NOT NULL DEFAULT '' CHECK (char_length(profile_url)<=1000),
 reason text NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 2000),
 status text NOT NULL DEFAULT 'suggested' CHECK (status IN ('suggested','informed','interested','applied','interview','offer','closed')),
 notes text NOT NULL DEFAULT '' CHECK (char_length(notes)<=2000),
 follow_up date,
 version bigint NOT NULL DEFAULT 1,
 created_by bigint REFERENCES users(id) ON DELETE SET NULL,
 updated_by bigint REFERENCES users(id) ON DELETE SET NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX team_student_followups_identity_idx ON team_student_followups(job_id,lower(student_ref));
CREATE INDEX team_student_followups_creator_idx ON team_student_followups(created_by);
CREATE INDEX team_student_followups_updater_idx ON team_student_followups(updated_by);
CREATE INDEX team_student_followups_queue_idx ON team_student_followups(updated_at DESC,id DESC);
CREATE INDEX team_student_followups_due_idx ON team_student_followups(follow_up,id) WHERE follow_up IS NOT NULL AND status NOT IN ('offer','closed');
CREATE TABLE team_student_followup_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 followup_id bigint NOT NULL REFERENCES team_student_followups(id) ON DELETE CASCADE,
 actor_id bigint REFERENCES users(id) ON DELETE SET NULL,
 action text NOT NULL CHECK (action IN ('created','updated','contacted')),
 student_name text NOT NULL,
 profile_url text NOT NULL,
 reason text NOT NULL,
 status text NOT NULL,
 notes text NOT NULL,
 follow_up date,
 version bigint NOT NULL,
 channel text NOT NULL DEFAULT '',
 contacted_at timestamptz,
 contact_note text NOT NULL DEFAULT '',
 request_id text,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(followup_id,version),
 UNIQUE(followup_id,request_id)
);
CREATE INDEX team_student_followup_events_actor_idx ON team_student_followup_events(actor_id);
