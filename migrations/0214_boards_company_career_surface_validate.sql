-- Validate separately so the catalogue stays readable/writable during the scan.
ALTER TABLE boards VALIDATE CONSTRAINT boards_surface_check;
