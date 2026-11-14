DROP INDEX IF EXISTS agent_session_turns_idempotency_idx;
ALTER TABLE agent_session_turns
    DROP CONSTRAINT IF EXISTS agent_session_turns_key_hash_check,
    DROP CONSTRAINT IF EXISTS agent_session_turns_idempotency_key_check,
    DROP COLUMN IF EXISTS interrupt_requested_at,
    DROP COLUMN IF EXISTS request_hash,
    DROP COLUMN IF EXISTS idempotency_key;
