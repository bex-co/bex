ALTER TABLE agent_sessions
    DROP COLUMN IF EXISTS failure_cause,
    DROP COLUMN IF EXISTS sandbox_wait_since;
