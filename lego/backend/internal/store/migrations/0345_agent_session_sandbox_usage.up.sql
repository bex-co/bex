-- w6/m566 t004: an agent session's outcome.cost prices what the bill charges
-- for its sandboxes. From sandboxUsage.agentBilledFrom (pricing.yaml) an agent
-- session's sandbox is billed for the CPU it uses and the memory it is
-- allocated, as a /v1/sandboxes sandbox is, instead of the weighted meter
-- compute_milli_seconds totals. These are the session's running totals of
-- those two billed kinds, kept beside compute_milli_seconds for the same
-- reason it is kept: usage_hourly is compacted, so the figure cannot be summed
-- from it later. Only rows not marked usage_hourly.unbilled count: the meter
-- marks every agent sandbox's CPU and memory before the switch, which the
-- weighted meter paid for.
ALTER TABLE agent_session_sandboxes
    ADD COLUMN cpu_seconds bigint NOT NULL DEFAULT 0 CHECK (cpu_seconds >= 0),
    ADD COLUMN memory_gb_seconds bigint NOT NULL DEFAULT 0 CHECK (memory_gb_seconds >= 0);

-- The binding seeds all three totals from what the meters already wrote for
-- the sandbox, under the meters' per-sandbox lock (0206).
CREATE OR REPLACE FUNCTION agent_session_bind_sandbox() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.sandbox_id = '' OR NEW.sandbox_id IS NOT DISTINCT FROM OLD.sandbox_id THEN
        RETURN NEW;
    END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended(NEW.workspace_id || '|' || NEW.sandbox_id, 0));
    INSERT INTO agent_session_sandboxes (workspace_id, sandbox_id, session_id, turn, compute_milli_seconds, cpu_seconds, memory_gb_seconds)
    SELECT NEW.workspace_id, NEW.sandbox_id, NEW.id, NEW.turns,
           COALESCE(SUM(quantity) FILTER (WHERE kind = 'sandbox_compute_seconds'), 0),
           COALESCE(SUM(quantity) FILTER (WHERE kind = 'sandbox_cpu_seconds' AND NOT unbilled), 0),
           COALESCE(SUM(quantity) FILTER (WHERE kind = 'sandbox_memory_gb_seconds' AND NOT unbilled), 0)
    FROM usage_hourly
    WHERE resource_kind = 'sandbox' AND workspace_id = NEW.workspace_id AND service_id = NEW.sandbox_id
    ON CONFLICT (workspace_id, sandbox_id) DO NOTHING;
    RETURN NEW;
END;
$$;
