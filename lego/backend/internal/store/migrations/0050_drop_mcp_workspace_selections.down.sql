CREATE TABLE mcp_workspace_selections (
    session_id text NOT NULL,
    subject text NOT NULL,
    workspace_id text NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, subject)
);

CREATE INDEX mcp_workspace_selections_workspace_idx
    ON mcp_workspace_selections (workspace_id);
