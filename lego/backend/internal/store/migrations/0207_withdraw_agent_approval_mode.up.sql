-- ADR100 approval mode is withdrawn (user decision 2026-11-07, .pm/DO_NOT_DO.md
-- #APPROVAL-MODE): cloud agent sessions stay unattended. This undoes 0205
-- exactly: the decision table and its mirror columns go, and the session
-- state notification goes back to firing on phase changes only. 0205 stays in
-- the history because it was applied.

DROP TRIGGER agent_sessions_notify_state ON agent_sessions;
CREATE OR REPLACE FUNCTION notify_agent_session_state() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' OR OLD.phase IS DISTINCT FROM NEW.phase THEN
        PERFORM pg_notify('agent_session_state', NEW.id);
    END IF;
    RETURN NULL;
END;
$$;
CREATE TRIGGER agent_sessions_notify_state
    AFTER INSERT OR UPDATE OF phase ON agent_sessions
    FOR EACH ROW EXECUTE FUNCTION notify_agent_session_state();

DROP TRIGGER agent_sessions_cancel_decisions ON agent_sessions;
DROP FUNCTION cancel_agent_session_decisions();

ALTER TABLE agent_environments
    DROP COLUMN approval_bypass_allowed,
    DROP COLUMN approval_mode;

ALTER TABLE agent_sessions
    DROP COLUMN approval_mode,
    DROP COLUMN pending_decision_count,
    DROP COLUMN pending_decision_kind,
    DROP COLUMN pending_decision_id;

DROP TABLE agent_session_decisions;
