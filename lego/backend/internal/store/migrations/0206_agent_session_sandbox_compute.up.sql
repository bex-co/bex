-- w9/m178 (ADR094 D7): attribute sandbox compute to the agent session that ran
-- it, so outcome.cost can price it (computeUSD) next to modelUSD.
--
-- The compute meter (0061) keys its cursor and usage rows by sandbox id, and a
-- session's turns each run in a fresh sandbox whose id the session row keeps
-- only while it lives. This table is the durable sandbox -> session map, with a
-- running total of the weighted milli-vCPU seconds the meter accrued for that
-- sandbox. usage_hourly is pruned, so the total cannot be summed from it later.
CREATE TABLE agent_session_sandboxes (
    workspace_id          text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    sandbox_id            text        NOT NULL,
    session_id            text        NOT NULL REFERENCES agent_sessions (id) ON DELETE CASCADE,
    turn                  int         NOT NULL,
    compute_milli_seconds bigint      NOT NULL DEFAULT 0 CHECK (compute_milli_seconds >= 0),
    created_at            timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, sandbox_id)
);

CREATE INDEX agent_session_sandboxes_session_idx ON agent_session_sandboxes (session_id);

-- Bind a sandbox to its session whenever the session row adopts it, on every
-- path that writes sandbox_id (first dispatch, re-dispatch). The meter may
-- already have accrued for it, so the new row starts from what usage_hourly
-- holds. The advisory lock is the meter's own per-sandbox key, so an accrual
-- running concurrently either lands in that sum or updates the new row, never
-- neither.
CREATE FUNCTION agent_session_bind_sandbox() RETURNS trigger
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

CREATE TRIGGER agent_session_bind_sandbox
    AFTER INSERT OR UPDATE OF sandbox_id ON agent_sessions
    FOR EACH ROW EXECUTE FUNCTION agent_session_bind_sandbox();

-- Sandboxes live now are attributed from here on; earlier, already-reaped
-- sandboxes stay unattributed (their sessions read computeUSD "0").
INSERT INTO agent_session_sandboxes (workspace_id, sandbox_id, session_id, turn, compute_milli_seconds)
SELECT s.workspace_id, s.sandbox_id, s.id, s.turns,
       COALESCE((SELECT SUM(u.quantity) FROM usage_hourly u
                 WHERE u.resource_kind = 'sandbox' AND u.kind = 'sandbox_compute_seconds'
                   AND u.workspace_id = s.workspace_id AND u.service_id = s.sandbox_id), 0)
FROM agent_sessions s
WHERE s.sandbox_id <> ''
ON CONFLICT (workspace_id, sandbox_id) DO NOTHING;
