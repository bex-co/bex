-- Reverse w5/m103's person/machine discriminator. Dropping the column returns
-- every bound API key to looking like an ordinary member: listed on the Team
-- surface, counted against the seat cap, and mutable through the member verbs.
-- The bindings themselves are unaffected — keys keep working either way.
DROP INDEX IF EXISTS tenant_members_people_idx;

ALTER TABLE tenant_members
  DROP CONSTRAINT IF EXISTS tenant_members_kind_check;

ALTER TABLE tenant_members
  DROP COLUMN IF EXISTS kind;
