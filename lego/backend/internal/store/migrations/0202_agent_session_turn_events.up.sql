-- w21/m260 (ADR094 D9): one durable row per turn that ended succeeded or
-- failed, the source of the agent_session_turn_succeeded and
-- agent_session_turn_failed workspace webhooks. A trigger on the phase change
-- writes it, so every terminalizing path (the Completer, the model-spend
-- refusal, the dispatch failure, the structured-output check) is covered by one
-- choke point. The row carries only the key; the webhook worker reads the
-- outcome from the session when it delivers. recorded_at is the feed cursor
-- (the same shape as service_event_facts), so dispatch tails insertion order.
CREATE TABLE agent_session_turn_events (
    session_id   text        NOT NULL REFERENCES agent_sessions (id) ON DELETE CASCADE,
    turn         int         NOT NULL CHECK (turn > 0),
    phase        text        NOT NULL CHECK (phase IN ('completed', 'failed')),
    workspace_id text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    recorded_at  timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (session_id, turn, phase)
);

CREATE INDEX agent_session_turn_events_feed_idx
    ON agent_session_turn_events (recorded_at, session_id, turn);

CREATE FUNCTION record_agent_session_turn_event() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO agent_session_turn_events (session_id, turn, phase, workspace_id)
    VALUES (NEW.id, GREATEST(NEW.turns, 1), NEW.phase, NEW.workspace_id)
    ON CONFLICT DO NOTHING;
    RETURN NULL;
END;
$$;

CREATE TRIGGER agent_sessions_turn_event
    AFTER UPDATE OF phase ON agent_sessions
    FOR EACH ROW
    WHEN (NEW.phase IN ('completed', 'failed') AND OLD.phase IS DISTINCT FROM NEW.phase)
    EXECUTE FUNCTION record_agent_session_turn_event();
