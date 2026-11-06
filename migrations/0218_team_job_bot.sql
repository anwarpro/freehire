-- Expand the preset vocabulary before deploying the team bot. Existing rows remain valid.
ALTER TABLE assistant_sessions DROP CONSTRAINT assistant_sessions_preset_check;
ALTER TABLE assistant_sessions ADD CONSTRAINT assistant_sessions_preset_check
 CHECK (preset IN ('chat','tailor','profile','browse','interview','debrief','apply','team_jobs')) NOT VALID;
