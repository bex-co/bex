-- w9/m481 (docs/ADR024-members.md § An API key acts with its creator's role):
-- an API key acts with its creator's current member role in the workspace it
-- was minted in, as Render's keys do ("Your key's access adjusts to match your
-- new member role", render.com/docs/api-keys).
--
-- created_by names that creator on the key's own machine row, so a role change
-- re-seats the creator's keys in the same transaction as the creator's row and
-- enqueues each key's intent in membership_role_reconciliations
-- (store.enqueueRoleReconciliation). Hydra's bex.co/created-by stamp stays the
-- record bex-api's request-time ceiling reads; this column is its index here.
--
-- Existing machine rows start NULL: SQL cannot read Hydra, so bex-api fills
-- them once at startup from the live key list (store.RecordKeyCreators, the
-- w5/m103 backfill's pattern). A key with no stamp at all stays NULL and keeps
-- its Developer binding (invariant I3).
ALTER TABLE tenant_members
  ADD COLUMN IF NOT EXISTS created_by text;

ALTER TABLE tenant_members
  DROP CONSTRAINT IF EXISTS tenant_members_created_by_check;

-- Only a key has a creator; a person's row never carries one.
ALTER TABLE tenant_members
  ADD CONSTRAINT tenant_members_created_by_check CHECK (created_by IS NULL OR kind = 'machine');

CREATE INDEX IF NOT EXISTS tenant_members_key_creator_idx
  ON tenant_members (tenant_id, created_by) WHERE kind = 'machine' AND created_by IS NOT NULL;
