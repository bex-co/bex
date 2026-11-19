ALTER TABLE agent_session_turns
    DROP CONSTRAINT IF EXISTS agent_session_turns_retention_cause_check,
    DROP CONSTRAINT IF EXISTS agent_session_turns_checkout_check,
    DROP COLUMN IF EXISTS retention_cause,
    DROP COLUMN IF EXISTS unpushed_commits,
    DROP COLUMN IF EXISTS checkout_dirty;
