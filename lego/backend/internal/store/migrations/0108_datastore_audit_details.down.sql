ALTER TABLE audit_events
    DROP COLUMN IF EXISTS high_availability_enabled,
    DROP COLUMN IF EXISTS connection_pool_enabled,
    DROP COLUMN IF EXISTS disk_size_gb,
    DROP COLUMN IF EXISTS maxmemory_policy,
    DROP COLUMN IF EXISTS persistence_mode;
