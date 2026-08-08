-- w1/m65 F2: a GitHub App installation may be connected by at most one
-- workspace. The App JWT can look up every installation of itself, so the
-- connect callback cannot treat a GetInstallation success as ownership proof;
-- enforcing a unique installation->workspace binding at the schema level (plus
-- the application-layer check in internal/github) prevents one tenant claiming
-- another tenant's installation and minting tokens for its repositories.

-- Self-heal any PRE-EXISTING duplicate bindings before enforcing uniqueness — a
-- duplicate is the very cross-tenant attach this index closes, and without this
-- the CREATE UNIQUE INDEX fails on historical data and dirties the migration.
-- Deterministic first-writer-wins: keep the earliest-created binding per
-- installation (created_at, then workspace_id as a stable tiebreaker), drop the
-- rest — matching the service-layer unique-binding rule. A no-op on a fresh DB.
DELETE FROM git_connections a
USING git_connections b
WHERE a.installation_id = b.installation_id
  AND (a.created_at, a.workspace_id) > (b.created_at, b.workspace_id);

CREATE UNIQUE INDEX IF NOT EXISTS git_connections_installation_idx
  ON git_connections (installation_id);
