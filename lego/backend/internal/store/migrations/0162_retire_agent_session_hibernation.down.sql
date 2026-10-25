-- Re-adds 0073's hibernation columns, phases and index for a rollback to a
-- binary that still reads them. Every row comes back unpinned with no snapshot;
-- the up migration's phase conversion is not reversed (those rows were already
-- valid terminal sessions).
ALTER TABLE agent_sessions
    ADD COLUMN pinned         boolean     NOT NULL DEFAULT false,
    ADD COLUMN snapshot_ref   text        NOT NULL DEFAULT '',
    ADD COLUMN snapshot_bytes bigint      NOT NULL DEFAULT 0,
    ADD COLUMN snapshot_sha   text        NOT NULL DEFAULT '',
    ADD COLUMN hibernated_at  timestamptz,
    ADD COLUMN retain_until   timestamptz;

ALTER TABLE agent_sessions DROP CONSTRAINT agent_sessions_phase_check;
ALTER TABLE agent_sessions ADD CONSTRAINT agent_sessions_phase_check
    CHECK (phase IN ('creating', 'running', 'resuming', 'redispatching',
                     'hibernating', 'hibernated',
                     'completed', 'failed', 'canceling', 'canceled'));

CREATE INDEX agent_sessions_hibernated_idx
    ON agent_sessions (retain_until)
    WHERE phase = 'hibernated' AND snapshot_ref <> '';
