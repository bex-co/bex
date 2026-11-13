DROP INDEX IF EXISTS sandbox_builds_with_image;
DROP INDEX IF EXISTS sandbox_groups_live_name;
ALTER TABLE sandbox_groups ADD CONSTRAINT sandbox_groups_workspace_name_key UNIQUE (workspace_id, name);
DROP INDEX IF EXISTS sandbox_groups_deleting;
ALTER TABLE sandbox_groups DROP CONSTRAINT IF EXISTS sandbox_groups_default_not_deleting;
ALTER TABLE sandbox_groups DROP COLUMN IF EXISTS deleting_at;
