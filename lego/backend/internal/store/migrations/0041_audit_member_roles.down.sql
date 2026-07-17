ALTER TABLE audit_events
    DROP COLUMN IF EXISTS role_from,
    DROP COLUMN IF EXISTS role_to;
