-- ADR047 D4 / w3/m41: phase-1 delivery + evidence + steering on the durable
-- agent session. A completed session carries the pushed head SHA, the draft PR
-- it opened, a bounded Codex-style evidence extract, a running turn count, and
-- the last delivery mode (resume vs re-dispatch) — all first-class so list/get
-- can project them without reading the sandbox.
ALTER TABLE agent_sessions
    ADD COLUMN head_sha       text  NOT NULL DEFAULT '',
    ADD COLUMN pr_url         text  NOT NULL DEFAULT '',
    ADD COLUMN pr_number      int   NOT NULL DEFAULT 0,
    ADD COLUMN evidence       jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN turns          int   NOT NULL DEFAULT 0,
    ADD COLUMN delivery_mode  text  NOT NULL DEFAULT '',
    ADD COLUMN failure_reason text  NOT NULL DEFAULT '';

-- 'completed' is the phase-1 terminal success state; 'redispatching' is the
-- transient steering re-create window (see ADR047 D8).
ALTER TABLE agent_sessions DROP CONSTRAINT agent_sessions_phase_check;
ALTER TABLE agent_sessions ADD CONSTRAINT agent_sessions_phase_check
    CHECK (phase IN ('creating', 'running', 'resuming', 'redispatching',
                     'completed', 'failed', 'canceling', 'canceled'));
