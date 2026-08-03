ALTER TABLE agent_sessions DROP CONSTRAINT agent_sessions_phase_check;
ALTER TABLE agent_sessions ADD CONSTRAINT agent_sessions_phase_check
    CHECK (phase IN ('creating', 'running', 'resuming', 'failed', 'canceling', 'canceled'));

ALTER TABLE agent_sessions
    DROP COLUMN head_sha,
    DROP COLUMN pr_url,
    DROP COLUMN pr_number,
    DROP COLUMN evidence,
    DROP COLUMN turns,
    DROP COLUMN delivery_mode,
    DROP COLUMN failure_reason;
