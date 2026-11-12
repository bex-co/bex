ALTER TABLE agent_sessions
    DROP CONSTRAINT agent_sessions_takeover_check,
    DROP COLUMN takeover_head_ref,
    DROP COLUMN takeover_pr_number;
