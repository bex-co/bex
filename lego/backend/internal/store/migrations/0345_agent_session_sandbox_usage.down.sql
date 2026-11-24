-- Reverse 0345: the binding seeds the weighted total alone again (0206).
CREATE OR REPLACE FUNCTION agent_session_bind_sandbox() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.sandbox_id = '' OR NEW.sandbox_id IS NOT DISTINCT FROM OLD.sandbox_id THEN
        RETURN NEW;
    END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended(NEW.workspace_id || '|' || NEW.sandbox_id, 0));
    INSERT INTO agent_session_sandboxes (workspace_id, sandbox_id, session_id, turn, compute_milli_seconds)
    SELECT NEW.workspace_id, NEW.sandbox_id, NEW.id, NEW.turns,
           COALESCE((SELECT SUM(quantity) FROM usage_hourly
                     WHERE resource_kind = 'sandbox' AND kind = 'sandbox_compute_seconds'
                       AND workspace_id = NEW.workspace_id AND service_id = NEW.sandbox_id), 0)
    ON CONFLICT (workspace_id, sandbox_id) DO NOTHING;
    RETURN NEW;
END;
$$;

ALTER TABLE agent_session_sandboxes
    DROP COLUMN IF EXISTS memory_gb_seconds,
    DROP COLUMN IF EXISTS cpu_seconds;
