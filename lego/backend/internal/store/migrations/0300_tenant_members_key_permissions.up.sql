-- w9/m496 (docs/ADR024-members.md § An API key carries a chosen set of
-- permissions): a bex API key carries a chosen subset of Render's per-area
-- permissions, as Render's scoped keys do ("API keys limit access to resources
-- within a specific workspace, so you can grant each key only the permissions
-- it needs", render.com/docs/api-keys).
--
-- key_permissions is the key's grant on its own machine row, one
-- "permission:level" entry each (core.KeyGrant.Entries). NULL is All: every
-- permission its creator's role allows, which is what every key minted before
-- this column, and every key minted without naming permissions, keeps. An
-- empty array is Render's "Clear all". bex-api reads it uncached on every
-- request a key makes, so a narrowed grant applies on the key's next request.
ALTER TABLE tenant_members
  ADD COLUMN IF NOT EXISTS key_permissions text[];

ALTER TABLE tenant_members
  DROP CONSTRAINT IF EXISTS tenant_members_key_permissions_check;

-- Only a key has a grant; a person's row never carries one.
ALTER TABLE tenant_members
  ADD CONSTRAINT tenant_members_key_permissions_check CHECK (key_permissions IS NULL OR kind = 'machine');
