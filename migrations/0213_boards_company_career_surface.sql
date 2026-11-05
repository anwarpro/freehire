-- New curator source-management surface, without validating historical rows while
-- holding the short exclusive lock. Existing sources retain their original surface.
ALTER TABLE boards DROP CONSTRAINT boards_surface_check;
ALTER TABLE boards ADD CONSTRAINT boards_surface_check
    CHECK (surface IN ('web', 'telegram', 'discord', 'extension', 'cli', 'unknown', 'curator', 'company-career')) NOT VALID;
