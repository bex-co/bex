-- w21/m258 (ADR094 D4): idempotent agent-session create. A client that retries
-- a create with the same key within 24 h gets the first session back instead of
-- a second sandbox. The key is unique per workspace; the hash is the normalized
-- request, so the same key with a different request is a named 409. A key older
-- than 24 h is released by CreateAgentSession (a partial index cannot use now()).
ALTER TABLE agent_sessions
    ADD COLUMN idempotency_key text,
    ADD COLUMN idempotency_hash text,
    ADD CONSTRAINT agent_sessions_idempotency_key_check
        CHECK (idempotency_key IS NULL OR (char_length(idempotency_key) BETWEEN 1 AND 128 AND idempotency_hash IS NOT NULL));

CREATE UNIQUE INDEX agent_sessions_idempotency_key_idx
    ON agent_sessions (workspace_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
