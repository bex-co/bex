DROP INDEX IF EXISTS tenant_members_key_creator_idx;
ALTER TABLE tenant_members DROP CONSTRAINT IF EXISTS tenant_members_created_by_check;
ALTER TABLE tenant_members DROP COLUMN IF EXISTS created_by;
