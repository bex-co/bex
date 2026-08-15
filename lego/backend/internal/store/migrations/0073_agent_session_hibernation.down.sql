DROP INDEX IF EXISTS agent_sessions_hibernated_idx;

ALTER TABLE agent_sessions DROP CONSTRAINT agent_sessions_phase_check;
ALTER TABLE agent_sessions ADD CONSTRAINT agent_sessions_phase_check
    CHECK (phase IN ('creating', 'running', 'resuming', 'redispatching',
                     'completed', 'failed', 'canceling', 'canceled'));

ALTER TABLE agent_sessions
    DROP COLUMN pinned,
    DROP COLUMN snapshot_ref,
    DROP COLUMN snapshot_bytes,
    DROP COLUMN snapshot_sha,
    DROP COLUMN hibernated_at,
    DROP COLUMN retain_until;
