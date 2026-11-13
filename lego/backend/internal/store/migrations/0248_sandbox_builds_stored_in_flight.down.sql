DROP INDEX IF EXISTS sandbox_builds_stored;
CREATE INDEX sandbox_builds_stored ON sandbox_builds (workspace_id) INCLUDE (size_bytes)
    WHERE status = 'succeeded' AND deleted_at IS NULL;
