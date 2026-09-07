CREATE TABLE agent_session_dispatches (
    session_id text NOT NULL,
    turn integer NOT NULL,
    workspace_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    next_check_at timestamptz NOT NULL DEFAULT now() + interval '15 minutes',
    abandoned boolean NOT NULL DEFAULT false,
    previous_sandbox_id text NOT NULL DEFAULT '',
    legacy boolean NOT NULL DEFAULT false,
    PRIMARY KEY (session_id, turn)
);
CREATE INDEX agent_session_dispatches_due ON agent_session_dispatches (abandoned, next_check_at);
-- Preserve pre-upgrade missing-bind sessions; their pods have no turn label.
INSERT INTO agent_session_dispatches (session_id, turn, workspace_id, created_at, next_check_at, legacy)
SELECT id, turns, workspace_id, updated_at, updated_at + interval '15 minutes', true
FROM agent_sessions WHERE sandbox_id = '' AND phase IN ('creating', 'redispatching');
