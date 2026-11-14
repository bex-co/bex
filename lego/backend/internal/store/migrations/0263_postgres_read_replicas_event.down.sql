ALTER TABLE audit_events
    DROP COLUMN IF EXISTS read_replicas_from,
    DROP COLUMN IF EXISTS read_replicas_to;
