ALTER TABLE audit_events DROP COLUMN IF EXISTS project_from;
ALTER TABLE audit_events DROP COLUMN IF EXISTS project_to;
ALTER TABLE audit_events DROP COLUMN IF EXISTS environment_from;
ALTER TABLE audit_events DROP COLUMN IF EXISTS environment_to;
