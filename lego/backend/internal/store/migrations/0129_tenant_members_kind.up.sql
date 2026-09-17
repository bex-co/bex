-- w5/m103: tell a person from a machine in tenant_members.
--
-- BindClient writes a tenant_members row keyed by the API key's Hydra client id
-- (role 'developer') so the key resolves to its workspace and authorizes there.
-- Nothing distinguished that row from a human's, so a bound key was listed as a
-- member, consumed a seat against the plan cap, and — verified live on dev-5,
-- 2026-09-17 — could be PROMOTED to admin through the Team members surface,
-- which wrote a real workspace:admin tuple for the machine subject.
--
-- The subject string cannot be the discriminator: a Kratos identity id and a
-- Hydra client id are both UUIDs, so any shape test would be a guess. Hence an
-- explicit column, written at bind time.
--
-- Reads split by intent, NOT by table:
--   * "who are the people here" (member list, seat count, admin count) filters
--     kind = 'user';
--   * "may this subject act here" (GetTenantMember, IsMember, TenantForIdentity,
--     TenantForKey) does NOT filter — a machine binding is exactly what makes an
--     API key work, and hiding it from those reads would break every key.
--
-- Existing rows default to 'user'. They cannot be classified in SQL (Hydra is
-- the only registry of client ids), so bex-api reconciles them once at startup
-- against the live client list — see store.MarkMachineMemberships and the
-- backfill in cmd/api/main.go. Until that runs, a pre-existing key keeps
-- looking like a member, which is exactly today's behavior: the migration
-- alone is never a regression.
ALTER TABLE tenant_members
  ADD COLUMN IF NOT EXISTS kind text NOT NULL DEFAULT 'user';

ALTER TABLE tenant_members
  DROP CONSTRAINT IF EXISTS tenant_members_kind_check;

ALTER TABLE tenant_members
  ADD CONSTRAINT tenant_members_kind_check CHECK (kind IN ('user', 'machine'));

-- The member/seat reads all filter on (tenant_id, kind); the leading column is
-- already the PK's, so this partial index serves the common "people only" scan
-- without a second full index.
CREATE INDEX IF NOT EXISTS tenant_members_people_idx
  ON tenant_members (tenant_id) WHERE kind = 'user';
