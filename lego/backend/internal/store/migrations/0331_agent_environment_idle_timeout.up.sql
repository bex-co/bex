-- w6/m563: an Enterprise workspace's own idle timeout for the agent sessions
-- people start in it, as Devin's Enterprise customers can set 5 to 120
-- minutes (docs.devin.ai/admin/billing/usage, read 2026-11-22). NULL, every
-- existing row and a workspace with no row keep the platform's 5 minutes; a
-- value applies only while the workspace is on Enterprise. Automations' and
-- child sessions' sandboxes close at their harvest whatever it says.
ALTER TABLE agent_environments
    ADD COLUMN idle_timeout_minutes integer
        CHECK (idle_timeout_minutes BETWEEN 5 AND 120);
