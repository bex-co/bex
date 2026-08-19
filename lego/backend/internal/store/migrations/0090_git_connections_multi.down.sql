-- Reverse w5/m74 (ADR075): restore the one-connection-per-workspace shape.
--
-- Guarded: a workspace that acquired a SECOND installation under the new model
-- cannot collapse back to a workspace-keyed primary key without losing data, so
-- refuse the down-migration rather than silently drop rows. On a table that never
-- gained a second per-workspace row (the migrate-up-then-down case, and every
-- single-connection production workspace) this is a no-op and the revert proceeds.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM git_connections
        GROUP BY workspace_id
        HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'cannot revert git_connections to one-per-workspace: a workspace holds multiple installations (ADR075); disconnect extras first';
    END IF;
END $$;

ALTER TABLE git_connections DROP CONSTRAINT git_connections_pkey;
DROP INDEX IF EXISTS git_connections_workspace_idx;
ALTER TABLE git_connections ADD PRIMARY KEY (workspace_id);
CREATE UNIQUE INDEX IF NOT EXISTS git_connections_installation_idx
  ON git_connections (installation_id);
