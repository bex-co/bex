-- w5/m287: Render sandbox names (Render CLI v2.36 `sandboxes create --name`,
-- `list --name`). A name is a label of at most 64 characters, not unique, and
-- never `sbx-…`, so it cannot live in OpenSandbox metadata, which the server
-- persists as Kubernetes labels (63 characters, a restricted alphabet). The
-- row is written before the sandbox is created, keyed by the public id bex
-- mints, and pruned with the sandbox.
CREATE TABLE sandbox_names (
    workspace_id text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    sandbox_id   text        NOT NULL,
    name         text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 64),
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, sandbox_id)
);

CREATE INDEX sandbox_names_by_name ON sandbox_names (workspace_id, name);
