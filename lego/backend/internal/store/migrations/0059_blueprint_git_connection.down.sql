-- 0059 down
DROP TABLE IF EXISTS blueprint_syncs;
ALTER TABLE blueprints
    DROP COLUMN IF EXISTS path,
    DROP COLUMN IF EXISTS auto_sync,
    DROP COLUMN IF EXISTS last_sync_at;
