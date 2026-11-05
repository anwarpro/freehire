-- A missing deadline means the source did not state one. Existing jobs stay unchanged.
ALTER TABLE jobs ADD COLUMN application_deadline timestamptz;
