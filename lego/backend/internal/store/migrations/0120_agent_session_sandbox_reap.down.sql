DROP INDEX IF EXISTS agent_sessions_sandbox_reap_idx;
ALTER TABLE agent_sessions
    DROP COLUMN IF EXISTS sandbox_reap_attempts,
    DROP COLUMN IF EXISTS sandbox_reap_after;
