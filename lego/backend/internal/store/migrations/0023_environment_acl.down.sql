ALTER TABLE environments
    DROP COLUMN IF EXISTS ip_allow_list,
    DROP COLUMN IF EXISTS network_isolation_enabled,
    DROP COLUMN IF EXISTS protected_status;
