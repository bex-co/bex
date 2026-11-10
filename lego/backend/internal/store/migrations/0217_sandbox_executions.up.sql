-- w32/m298: Render's sandbox execution history (`GET /sandboxes/{id}/execs`).
-- One row per command run or file transfer, written when its connect token is
-- minted (Render's `startedAt` is the token provisioning time) and stopped
-- when it ends. Rows are pruned with their sandbox, like sandbox_names.
CREATE TABLE sandbox_executions (
    id           text        PRIMARY KEY,
    workspace_id text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    sandbox_id   text        NOT NULL,
    type         text        NOT NULL CHECK (type IN ('runs', 'files')),
    operation    text        NOT NULL CHECK (operation IN ('sync', 'stream', 'upload', 'download')),
    command      text        NOT NULL DEFAULT '',
    subject      text        NOT NULL,
    started_at   timestamptz NOT NULL,
    stopped_at   timestamptz,
    exit_code    integer
);

CREATE INDEX sandbox_executions_by_sandbox ON sandbox_executions (workspace_id, sandbox_id, started_at DESC, id DESC);
