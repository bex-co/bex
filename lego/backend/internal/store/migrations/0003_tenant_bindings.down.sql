DROP INDEX IF EXISTS tenants_owner_identity_idx;
ALTER TABLE tenants DROP COLUMN IF EXISTS owner_identity_id;
