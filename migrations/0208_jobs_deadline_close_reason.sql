-- migrate: no-transaction
-- Widen without scanning the jobs table under an exclusive lock.
ALTER TABLE public.jobs
    DROP CONSTRAINT IF EXISTS jobs_closed_reason_check;

ALTER TABLE public.jobs
    ADD CONSTRAINT jobs_closed_reason_check CHECK (closed_reason = ANY (ARRAY[
        ''::text,                     -- closed before the column existed; unknown
        'unseen'::text,               -- dropped off a board we crawl (CloseUnseenJobs / …BySource)
        'feed_removed'::text,         -- a streaming source reported it taken down
        'moderated'::text,            -- a moderator closed it
        'probe_expired'::text,        -- two consecutive expired liveness probes
        'expired'::text,              -- age rule: no close signal exists for this source
        'board_unreachable'::text,    -- chronic-board safety net: no successful crawl in 60+ days
        'feed_empty'::text,           -- empty-feed safety net: crawls succeed, feed carries nothing
        'source_misattributed'::text, -- what we stored about it is wrong and unrepairable in place
        'application_deadline'::text, -- the stated application window ended
        'employer_closed'::text       -- a verified employer closed their own self-published vacancy
    ])) NOT VALID;

ALTER TABLE public.jobs
    VALIDATE CONSTRAINT jobs_closed_reason_check;
