-- w1/m33: typed member-role detail for the team-membership audit verbs —
-- members.ChangeRole records old→new; members.Invite / members.AcceptInvite
-- record the granted role in role_to alone. Same structural guard as
-- plan_from/plan_to (0037): typed nullable columns, no free-form details
-- object, so no secret can reach the feed. Values come from the closed
-- members.Roles ladder.
--
-- Renumbered 0040→0041 (w1/m46 closeout of the collision
-- TestMigrationNumbersAreUnique caught): a concurrent milestone shipped
-- 0040_deploy_commit_author_at first, and production applied THAT as version
-- 40. IF NOT EXISTS keeps this idempotent for databases that already applied
-- this file's content while it was numbered 0040 (dev-N environments) — see
-- 0012_audit_target.up.sql for why a silently skipped ALTER is the failure
-- mode to design against.
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS role_from text,
    ADD COLUMN IF NOT EXISTS role_to text;
