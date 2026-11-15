ALTER TABLE agent_session_turns
    DROP CONSTRAINT IF EXISTS agent_session_turns_retention_check,
    DROP COLUMN IF EXISTS start_note,
    DROP COLUMN IF EXISTS retained_until,
    DROP COLUMN IF EXISTS retention;
DROP TABLE IF EXISTS agent_session_retained_images;
