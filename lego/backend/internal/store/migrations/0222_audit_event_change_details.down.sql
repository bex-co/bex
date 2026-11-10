ALTER TABLE audit_events
    DROP COLUMN IF EXISTS disk_size_from_gb,
    DROP COLUMN IF EXISTS maintenance_uri_from,
    DROP COLUMN IF EXISTS maintenance_uri_to,
    DROP COLUMN IF EXISTS auto_deploy_reason,
    DROP COLUMN IF EXISTS auto_deploy_trigger;
