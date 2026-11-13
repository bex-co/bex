-- w2/m359: deleting a sandbox group, as Render's `sandbox-groups delete`
-- does: it terminates the group's sandboxes and deletes its images. The delete
-- first marks the group (deleting_at), which hides it from every read and
-- admits no new sandbox, then finishes the teardown and removes the row; a
-- sweeper finishes any delete a bex-api restart interrupted. The workspace's
-- default group is never marked.
ALTER TABLE sandbox_groups ADD COLUMN deleting_at timestamptz;
ALTER TABLE sandbox_groups ADD CONSTRAINT sandbox_groups_default_not_deleting
    CHECK (NOT (is_default AND deleting_at IS NOT NULL));
CREATE INDEX sandbox_groups_deleting ON sandbox_groups (deleting_at) WHERE deleting_at IS NOT NULL;

-- A deleted group's name is free at once, even while its teardown finishes.
ALTER TABLE sandbox_groups DROP CONSTRAINT sandbox_groups_workspace_name_key;
CREATE UNIQUE INDEX sandbox_groups_live_name ON sandbox_groups (workspace_id, name) WHERE deleting_at IS NULL;

-- A group or workspace delete lists the builds whose images it removes.
CREATE INDEX sandbox_builds_with_image ON sandbox_builds (workspace_id, sandbox_group_id) WHERE status <> 'failed';
