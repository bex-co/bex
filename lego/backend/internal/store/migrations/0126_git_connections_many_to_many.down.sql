-- Reverse w2/m162 (ADR078 §2): restore one-workspace-per-installation.
--
-- GUARDED, because reverting N:N is an operational procedure rather than a
-- rollback. A pre-revision binary reads the installation->workspace lookup as a
-- SINGLE-ROW query; with two rows for one installation it fails or returns an
-- arbitrary workspace, which on the push-webhook path means delivering a tenant's
-- deploy into the wrong scope. So refuse while any installation serves more than
-- one workspace, naming the offenders, rather than silently dropping bindings.
--
-- Un-deploy ordering is therefore: disconnect the extra bindings
-- (DELETE /v1/git/connections/{installationId}), then the binary, then this
-- migration. On a table that never gained a second binding this is a no-op and
-- the revert proceeds unchanged.
DO $$
DECLARE
    offenders text;
BEGIN
    SELECT string_agg(installation_id::text, ', ' ORDER BY installation_id)
      INTO offenders
      FROM (
        SELECT installation_id
          FROM git_connections
         GROUP BY installation_id
        HAVING count(*) > 1
      ) dup;

    IF offenders IS NOT NULL THEN
        RAISE EXCEPTION 'cannot revert git_connections to one-workspace-per-installation: installation(s) % serve multiple workspaces (ADR078 §2); disconnect the extra bindings first', offenders;
    END IF;
END $$;

ALTER TABLE git_connections DROP CONSTRAINT git_connections_pkey;
DROP INDEX IF EXISTS git_connections_installation_idx;
ALTER TABLE git_connections ADD PRIMARY KEY (installation_id);
CREATE INDEX IF NOT EXISTS git_connections_workspace_idx ON git_connections (workspace_id);
