DELETE FROM blueprint_syncs WHERE state = 'created' AND plan_path IS NOT NULL;
ALTER TABLE blueprints DROP COLUMN IF EXISTS resource_suffix, DROP COLUMN IF EXISTS existing_resources;
ALTER TABLE blueprint_syncs
    DROP COLUMN IF EXISTS required_manual_env_vars,
    DROP COLUMN IF EXISTS plan_path;
