-- w9/m385: workspace agent secret scopes (ADR098 D9). A workspace agent
-- secret's value lives only in OpenBao; this row says which sessions receive
-- it without naming it:
--   workspace — every session in the workspace;
--   repo      — sessions whose repository is `repo` (owner/name);
--   explicit  — only sessions that name it in secretNames (ADR098 D1).
-- A secret with no row is explicit: that is how secrets created before
-- scopes keep their behavior without a backfill across OpenBao. New secrets
-- get a 'workspace' row (Devin's organization secrets).
CREATE TABLE agent_secret_scopes (
    workspace_id text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name         text        NOT NULL CHECK (length(name) BETWEEN 1 AND 128),
    scope        text        NOT NULL CHECK (scope IN ('workspace', 'repo', 'explicit')),
    repo         text        NOT NULL DEFAULT '' CHECK (length(repo) <= 255),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, name),
    CHECK ((scope = 'repo') = (repo <> ''))
);

-- The secrets the session's latest dispatch applied, each with why
-- ([{"name":"X","source":"workspace"}]): names only, never a value.
ALTER TABLE agent_sessions
    ADD COLUMN applied_secrets jsonb NOT NULL DEFAULT '[]'::jsonb;
