-- Re-creates what 0205 added (ADR100), for a rollback of 0207 only.

CREATE TABLE agent_session_decisions (
    id           text        PRIMARY KEY,
    session_id   text        NOT NULL REFERENCES agent_sessions (id) ON DELETE CASCADE,
    workspace_id text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    turn         int         NOT NULL CHECK (turn > 0),
    -- request_id is the driver's own key for the request, so a retried POST
    -- returns the same decision instead of opening a second one.
    request_id   text        NOT NULL CHECK (length(request_id) BETWEEN 1 AND 128),
    kind         text        NOT NULL CHECK (kind IN ('approval', 'question')),
    title        text        NOT NULL CHECK (length(title) BETWEEN 1 AND 512),
    detail       jsonb       NOT NULL DEFAULT '{}'::jsonb,
    state        text        NOT NULL DEFAULT 'pending'
                             CHECK (state IN ('pending', 'allowed', 'denied', 'answered', 'expired', 'canceled')),
    answer       jsonb       NULL,
    answered_by  text        NULL,
    created_at   timestamptz NOT NULL DEFAULT clock_timestamp(),
    answered_at  timestamptz NULL,
    expires_at   timestamptz NOT NULL,
    UNIQUE (session_id, request_id)
);

-- The webhook feed tails decisions in insertion order (ADR094 D9's
-- agent_session_needs_input), like agent_session_turn_events.
CREATE INDEX agent_session_decisions_feed_idx ON agent_session_decisions (created_at, id);
-- The expiry sweep reads only open decisions.
CREATE INDEX agent_session_decisions_pending_idx ON agent_session_decisions (expires_at) WHERE state = 'pending';

ALTER TABLE agent_sessions
    ADD COLUMN pending_decision_id    text NULL
                                      REFERENCES agent_session_decisions (id) ON DELETE SET NULL,
    ADD COLUMN pending_decision_kind  text NULL CHECK (pending_decision_kind IN ('approval', 'question')),
    ADD COLUMN pending_decision_count int  NOT NULL DEFAULT 0 CHECK (pending_decision_count >= 0),
    ADD COLUMN approval_mode          text NOT NULL DEFAULT 'autonomous'
                                      CHECK (approval_mode IN ('autonomous', 'risky', 'all'));

ALTER TABLE agent_environments
    ADD COLUMN approval_mode           text    NOT NULL DEFAULT 'autonomous'
                                       CHECK (approval_mode IN ('autonomous', 'risky', 'all')),
    ADD COLUMN approval_bypass_allowed boolean NOT NULL DEFAULT true;

-- A turn that stops running (completed, failed, canceled, redispatched) cannot
-- act on an answer any more: one choke point resolves its open decisions as
-- canceled and clears the mirror, whichever path moved the phase. It runs
-- holding the session row lock and then touches decision rows, the lock order
-- every decision write follows (session first, then decisions).
CREATE FUNCTION cancel_agent_session_decisions() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.phase <> 'running' AND NEW.pending_decision_count > 0 THEN
        UPDATE agent_session_decisions
           SET state = 'canceled', answered_at = clock_timestamp()
         WHERE session_id = NEW.id AND state = 'pending';
        NEW.pending_decision_id := NULL;
        NEW.pending_decision_kind := NULL;
        NEW.pending_decision_count := 0;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER agent_sessions_cancel_decisions
    BEFORE UPDATE OF phase ON agent_sessions
    FOR EACH ROW
    WHEN (NEW.phase IS DISTINCT FROM OLD.phase)
    EXECUTE FUNCTION cancel_agent_session_decisions();

-- wait_agent_sessions and a decision await wake on a decision opening or
-- closing too: needs_input is a state change even though the phase stays
-- running, and closing a newer decision changes only the count.
CREATE OR REPLACE FUNCTION notify_agent_session_state() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' OR OLD.phase IS DISTINCT FROM NEW.phase
       OR OLD.pending_decision_id IS DISTINCT FROM NEW.pending_decision_id
       OR OLD.pending_decision_count IS DISTINCT FROM NEW.pending_decision_count THEN
        PERFORM pg_notify('agent_session_state', NEW.id);
    END IF;
    RETURN NULL;
END;
$$;

DROP TRIGGER agent_sessions_notify_state ON agent_sessions;
CREATE TRIGGER agent_sessions_notify_state
    AFTER INSERT OR UPDATE OF phase, pending_decision_id, pending_decision_count ON agent_sessions
    FOR EACH ROW EXECUTE FUNCTION notify_agent_session_state();
