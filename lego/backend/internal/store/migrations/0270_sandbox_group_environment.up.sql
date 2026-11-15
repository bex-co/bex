-- w22/m412: a sandbox group can be placed in a project environment, as Render's
-- dashboard lets a customer do ("In the Dashboard, you can optionally place a
-- group in a project environment", render.com/docs/sandboxes-creating, read
-- 2026-11-14; docs/ADR042-sandbox-cluster-substrate.md § Sandbox groups).
--
-- Membership only: the environment is a label the group carries and reads back
-- as its `environmentId`; who can see or use the group stays the workspace's.
-- A group in no environment (every group so far) has NULL. Deleting the
-- environment, or the project it sits in, keeps the group and clears the field,
-- as env_groups.environment_id does (0149). That the environment belongs to the
-- group's own workspace is the sandbox service's check, made when it places the
-- group through core.EnvironmentResolver, like every other resource kind.
ALTER TABLE sandbox_groups ADD COLUMN environment_id text REFERENCES environments (id) ON DELETE SET NULL;

-- An environment delete finds the groups it must clear; most groups are in none.
CREATE INDEX sandbox_groups_environment_idx ON sandbox_groups (environment_id) WHERE environment_id IS NOT NULL;
