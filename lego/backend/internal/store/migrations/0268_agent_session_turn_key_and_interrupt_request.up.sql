-- ADR047 D12. Two facts about a turn that bex-api, not the sandbox, must be the one to record.
--
-- The idempotency key and request digest of the direct send that started the
-- turn, written in the transaction that inserts the turn, so a retried keyed
-- send finds the turn it started instead of queueing a duplicate that would
-- run as a second one. A turn a queued message started carries no key: the
-- message keeps it. Unique per session, like the queue's.
--
-- When a person asked to stop the turn. The driver writes status
-- `interrupted` from the sandbox, which is untrusted, so the Completer honors
-- it only for a turn with a request recorded here, before the sentinel is
-- written. A request belongs to its turn and never covers the next.
--
-- The checks are added NOT VALID: every existing row has both columns NULL, so
-- it satisfies them, and skipping the validation scan keeps the lock brief.
-- New and updated rows are still checked.
ALTER TABLE agent_session_turns
    ADD COLUMN idempotency_key text,
    ADD COLUMN request_hash text,
    ADD COLUMN interrupt_requested_at timestamptz;

ALTER TABLE agent_session_turns
    ADD CONSTRAINT agent_session_turns_idempotency_key_check
        CHECK (idempotency_key IS NULL OR char_length(idempotency_key) BETWEEN 1 AND 128) NOT VALID,
    ADD CONSTRAINT agent_session_turns_key_hash_check
        CHECK ((idempotency_key IS NULL) = (request_hash IS NULL)) NOT VALID;

CREATE UNIQUE INDEX agent_session_turns_idempotency_idx
    ON agent_session_turns (session_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
