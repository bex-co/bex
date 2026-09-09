-- w5/m88: durable running-start timestamp for agent turns so lifecycle metrics
-- do not depend on agent_sessions.updated_at (which pin/archive also rewrite).
ALTER TABLE agent_session_turns
    ADD COLUMN started_at timestamptz;
