-- w21/m258 (ADR094 D6): wait_agent_sessions blocks server-side on this channel
-- instead of making the caller poll. Every phase change (and the insert that
-- starts a session) announces the session id; the payload is the id only, so
-- nothing sensitive rides the channel and a missed notification is recovered by
-- the waiter's periodic re-read.
CREATE FUNCTION notify_agent_session_state() RETURNS trigger
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
