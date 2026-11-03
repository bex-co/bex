ALTER TABLE agent_sessions
    DROP CONSTRAINT IF EXISTS agent_sessions_tags_check,
    DROP CONSTRAINT IF EXISTS agent_sessions_title_check,
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS title;
