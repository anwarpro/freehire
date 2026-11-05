-- Additive curriculum projection and private review state. Apply before app/tools.
CREATE TABLE curriculum_config (
 singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
 version bigint NOT NULL DEFAULT 1,
 payload jsonb NOT NULL,
 updated_by bigint REFERENCES users(id) ON DELETE SET NULL,
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX curriculum_config_actor_idx ON curriculum_config(updated_by);
INSERT INTO curriculum_config(singleton,payload) VALUES (true, '{"tracks": [{"id": "l1", "label": "L1 · Frontend fundamentals", "enabled": true, "minimum": 1, "skills": [{"name": "HTML", "aliases": ["html", "html5"]}, {"name": "CSS", "aliases": ["css", "css3"]}, {"name": "JavaScript", "aliases": ["javascript", "ecmascript"]}, {"name": "TypeScript", "aliases": ["typescript"]}, {"name": "React", "aliases": ["react", "reactjs"]}, {"name": "Web accessibility", "aliases": ["web accessibility", "wcag"]}]}, {"id": "l2", "label": "L2 · Backend development", "enabled": true, "minimum": 1, "skills": [{"name": "Node.js", "aliases": ["node.js", "nodejs"]}, {"name": "Express", "aliases": ["express.js", "expressjs"]}, {"name": "Databases", "aliases": ["postgresql", "mysql", "mongodb"]}, {"name": "Backend APIs", "aliases": ["rest api", "restful", "graphql"]}, {"name": "Backend engineering", "aliases": ["backend development", "back-end development", "backend engineer"]}]}, {"id": "python", "label": "Python · Software engineering", "enabled": true, "minimum": 1, "skills": [{"name": "Python", "aliases": ["python", "python3"]}, {"name": "Django", "aliases": ["django"]}, {"name": "FastAPI", "aliases": ["fastapi"]}, {"name": "Flask", "aliases": ["flask"]}, {"name": "Software testing", "aliases": ["pytest", "unittest"]}]}, {"id": "ai-ml", "label": "AI / ML", "enabled": true, "minimum": 1, "skills": [{"name": "Machine learning", "aliases": ["machine learning"]}, {"name": "Deep learning", "aliases": ["deep learning"]}, {"name": "ML frameworks", "aliases": ["pytorch", "tensorflow", "scikit-learn", "sklearn"]}, {"name": "Language models", "aliases": ["large language model", "llm", "natural language processing"]}, {"name": "Computer vision", "aliases": ["computer vision"]}]}, {"id": "data-analysis", "label": "Data analysis", "enabled": true, "minimum": 1, "skills": [{"name": "Data analysis", "aliases": ["data analysis", "data analytics"]}, {"name": "Data tools", "aliases": ["pandas", "numpy"]}, {"name": "Business intelligence", "aliases": ["power bi", "tableau"]}, {"name": "Data querying", "aliases": ["sql"]}, {"name": "Spreadsheets", "aliases": ["excel"]}]}, {"id": "competitive-programming", "label": "Competitive programming", "enabled": true, "minimum": 1, "skills": [{"name": "Competitive programming", "aliases": ["competitive programming", "icpc", "codeforces"]}, {"name": "Algorithms", "aliases": ["algorithms", "data structures"]}]}]}'::jsonb);
CREATE TABLE curriculum_config_events (
 version bigint PRIMARY KEY,
 payload jsonb NOT NULL,
 actor_id bigint REFERENCES users(id) ON DELETE SET NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX curriculum_config_events_actor_idx ON curriculum_config_events(actor_id);
INSERT INTO curriculum_config_events(version,payload) SELECT version,payload FROM curriculum_config;
CREATE TABLE curriculum_matches (
 job_id bigint PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,
 mapping_version bigint NOT NULL,
 input_hash text NOT NULL,
 evidence jsonb NOT NULL,
 suggested_tracks text[] NOT NULL DEFAULT '{}',
 reviewed_tracks text[],
 effective_tracks text[] GENERATED ALWAYS AS (COALESCE(reviewed_tracks,suggested_tracks)) STORED,
 reviewed_by bigint REFERENCES users(id) ON DELETE SET NULL,
 reviewed_at timestamptz,
 reviewed_hash text,
 reviewed_mapping_version bigint,
 review_notes text NOT NULL DEFAULT '' CHECK (char_length(review_notes)<=2000),
 review_version bigint NOT NULL DEFAULT 0,
 computed_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX curriculum_matches_reviewer_idx ON curriculum_matches(reviewed_by);
CREATE INDEX curriculum_matches_tracks_idx ON curriculum_matches USING gin(effective_tracks);
CREATE INDEX curriculum_matches_mapping_idx ON curriculum_matches(mapping_version,job_id DESC);
CREATE TABLE curriculum_review_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 job_id bigint NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
 actor_id bigint REFERENCES users(id) ON DELETE SET NULL,
 tracks text[],
 notes text NOT NULL CHECK (char_length(notes)<=2000),
 input_hash text NOT NULL,
 mapping_version bigint NOT NULL,
 version bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(job_id,version)
);
CREATE INDEX curriculum_review_events_actor_idx ON curriculum_review_events(actor_id);
CREATE TABLE curriculum_outbox (
 job_id bigint PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE
);
CREATE TABLE curriculum_scan_state (
 singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
 mapping_version bigint NOT NULL DEFAULT 1,
 cursor_id bigint NOT NULL DEFAULT 0,
 upper_id bigint NOT NULL DEFAULT 0,
 complete boolean NOT NULL DEFAULT false,
 processed bigint NOT NULL DEFAULT 0,
 last_run_at timestamptz,
 last_error text NOT NULL DEFAULT ''
);
INSERT INTO curriculum_scan_state(singleton) VALUES (true);
-- Queue only changes to the matcher input/privacy. Existing jobs are seeded in
-- bounded worker pages rather than a catalogue-wide migration INSERT.
CREATE FUNCTION queue_curriculum_job() RETURNS trigger LANGUAGE plpgsql AS $body$
BEGIN
 IF TG_OP='INSERT' OR OLD.title IS DISTINCT FROM NEW.title
    OR OLD.description IS DISTINCT FROM NEW.description
    OR OLD.is_private IS DISTINCT FROM NEW.is_private
    OR (OLD.closed_at IS NOT NULL AND NEW.closed_at IS NULL) THEN
   INSERT INTO curriculum_outbox(job_id) VALUES (NEW.id) ON CONFLICT DO NOTHING;
 END IF;
 RETURN NEW;
END;
$body$;
CREATE TRIGGER jobs_curriculum_changed AFTER INSERT OR UPDATE OF title,description,is_private,closed_at ON jobs
FOR EACH ROW EXECUTE FUNCTION queue_curriculum_job();
