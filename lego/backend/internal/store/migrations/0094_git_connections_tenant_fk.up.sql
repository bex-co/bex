-- codex-security geyRc8 F12: git_connections is workspace-owned metadata and
-- must disappear with its tenant. Remove any legacy orphan rows before adding
-- the FK so migration remains safe on existing installations.
DELETE FROM git_connections
WHERE NOT EXISTS (
    SELECT 1 FROM tenants WHERE tenants.id = git_connections.workspace_id
);

ALTER TABLE git_connections
ADD CONSTRAINT git_connections_workspace_fk
FOREIGN KEY (workspace_id) REFERENCES tenants(id) ON DELETE CASCADE;
