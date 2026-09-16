-- Deferred-teardown backoff for terminal sessions that still hold a sandbox.
-- A failing terminate advances sandbox_reap_after so retries back off without
-- ever dropping the row from the scan (w5/m99 t002). Eligibility is per-row:
-- sandbox_reap_after IS NULL OR <= now — never a now-relative updated_at window.
ALTER TABLE agent_sessions
    ADD COLUMN IF NOT EXISTS sandbox_reap_after timestamptz,
    ADD COLUMN IF NOT EXISTS sandbox_reap_attempts integer NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS agent_sessions_sandbox_reap_idx
    ON agent_sessions (sandbox_reap_after, updated_at, id)
    WHERE sandbox_id <> '' AND phase IN ('completed', 'failed', 'canceled');
