DROP INDEX IF EXISTS sandbox_groups_environment_idx;
ALTER TABLE sandbox_groups DROP COLUMN IF EXISTS environment_id;
