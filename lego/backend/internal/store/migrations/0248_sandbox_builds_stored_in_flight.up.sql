-- Round 24, S-3 (w5/m371): the per-workspace storage cap counts imports still
-- copying at their declared size, so the cap's sum reads pending and running
-- builds too. The partial index follows the query's condition.
DROP INDEX IF EXISTS sandbox_builds_stored;
CREATE INDEX sandbox_builds_stored ON sandbox_builds (workspace_id) INCLUDE (size_bytes)
    WHERE status <> 'failed' AND deleted_at IS NULL;
