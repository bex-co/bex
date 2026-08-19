-- w5/m74 (ADR075): relax git_connections from one-connection-per-workspace to N
-- installations per workspace, keeping one-workspace-per-installation.
--
-- BEFORE: PRIMARY KEY (workspace_id) capped a workspace at a single GitHub App
-- installation, so a team with repos under both an org and a member's personal
-- account could never see both in one repo picker — connecting the second
-- silently replaced the first (ON CONFLICT (workspace_id) DO UPDATE).
--
-- AFTER: PRIMARY KEY (installation_id) — an installation still belongs to at most
-- one workspace (this preserves the w1/m65 F2 cross-tenant-attach protection and
-- keeps WorkspaceForInstallation a function for fail-closed webhook scoping), but
-- a workspace may now hold many rows. A non-unique workspace index backs the
-- per-workspace list + owner lookup.
--
-- Metadata-only: production holds exactly one row per workspace by construction of
-- the old PRIMARY KEY, so promoting installation_id to the primary key needs no
-- dedup or backfill. The pre-existing unique index git_connections_installation_idx
-- (migration 0069) already guarantees installation_id is unique, so the promotion
-- cannot fail on live data.
ALTER TABLE git_connections DROP CONSTRAINT git_connections_pkey;

-- The unique index from 0069 already enforces installation_id uniqueness; make it
-- the primary key. Dropping the now-redundant unique index first lets ADD PRIMARY
-- KEY build its own supporting index without a duplicate.
DROP INDEX IF EXISTS git_connections_installation_idx;
ALTER TABLE git_connections ADD PRIMARY KEY (installation_id);

-- Back the per-workspace list + owner resolution (ListGitConnections /
-- GetGitConnectionByOwner). Non-unique: a workspace may hold many installations.
CREATE INDEX IF NOT EXISTS git_connections_workspace_idx ON git_connections (workspace_id);
