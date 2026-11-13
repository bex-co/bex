-- w3/m367: Render's delete-sandbox-build (round 7). A deleted build "no longer
-- appears in lists and cannot be retrieved or used to create sandboxes,
-- including by image name", and a repeated delete answers 204, so the row is
-- kept and marked rather than removed. Every read and every index that served
-- a live build now leaves deleted ones out, so the same name and digest can be
-- imported again.
ALTER TABLE sandbox_builds ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- Render's build failureCode (same round): a machine-readable reason, null
-- unless failed. Builds that failed before it read internal_error.
ALTER TABLE sandbox_builds ADD COLUMN IF NOT EXISTS failure_code text;

DROP INDEX IF EXISTS sandbox_builds_dedupe;
CREATE UNIQUE INDEX sandbox_builds_dedupe ON sandbox_builds (sandbox_group_id, name, source_digest)
    WHERE status <> 'failed' AND deleted_at IS NULL;

DROP INDEX IF EXISTS sandbox_builds_by_name;
CREATE INDEX sandbox_builds_by_name ON sandbox_builds (sandbox_group_id, name, created_at DESC)
    WHERE status = 'succeeded' AND deleted_at IS NULL;

DROP INDEX IF EXISTS sandbox_builds_stored;
CREATE INDEX sandbox_builds_stored ON sandbox_builds (workspace_id) INCLUDE (size_bytes)
    WHERE status = 'succeeded' AND deleted_at IS NULL;

-- The newest-first list serves live builds only; deleted rows stay for a
-- repeated delete's 204 and a group delete's registry sweep, which keeps
-- sandbox_builds_with_image unchanged.
DROP INDEX IF EXISTS sandbox_builds_newest;
CREATE INDEX sandbox_builds_newest ON sandbox_builds (sandbox_group_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
