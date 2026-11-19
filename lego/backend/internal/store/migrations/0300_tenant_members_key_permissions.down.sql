ALTER TABLE tenant_members DROP CONSTRAINT IF EXISTS tenant_members_key_permissions_check;
ALTER TABLE tenant_members DROP COLUMN IF EXISTS key_permissions;
