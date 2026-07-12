-- A workspace's GitHub App connection: which installation belongs to which
-- workspace (docs/ADR026-github-integration.md). One connection per workspace — the
-- workspace id is the primary key, so a re-connect upserts rather than
-- accumulating rows.
CREATE TABLE git_connections (
    workspace_id text PRIMARY KEY,
    installation_id bigint NOT NULL,
    account_login text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
