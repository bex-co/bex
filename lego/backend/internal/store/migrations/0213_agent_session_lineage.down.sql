DROP INDEX IF EXISTS agent_sessions_root_idx;
DROP INDEX IF EXISTS agent_sessions_parent_idx;
ALTER TABLE agent_sessions
    DROP COLUMN IF EXISTS depth,
    DROP COLUMN IF EXISTS root_session_id,
    DROP COLUMN IF EXISTS parent_session_id;
