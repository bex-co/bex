DROP INDEX IF EXISTS agent_sessions_idempotency_key_idx;
ALTER TABLE agent_sessions
    DROP CONSTRAINT IF EXISTS agent_sessions_idempotency_key_check,
    DROP COLUMN IF EXISTS idempotency_hash,
    DROP COLUMN IF EXISTS idempotency_key;
