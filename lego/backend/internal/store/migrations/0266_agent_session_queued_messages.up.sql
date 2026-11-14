-- ADR047 D12: a message sent to a running agent session is queued
-- instead of refused, and runs as its own steering turn once the session's
-- turn settles. One row is one message, in the order its position gives. The
-- queue dispatcher leases the head of a settled session's queue, steers the
-- session with it as the member who sent it, and the same transaction that
-- starts the turn records the delivery, so a message starts one turn only.
-- The table is new and not D11's agent_session_feedback: that one is keyed by
-- the GitHub object a PR event came from and its kind admits only PR kinds.
CREATE TABLE agent_session_queued_messages (
    id             text        PRIMARY KEY,
    session_id     text        NOT NULL REFERENCES agent_sessions (id) ON DELETE CASCADE,
    -- Sorts the queue first to last. Only the order matters, not the values: a
    -- move reassigns the pending rows' own values among them.
    position       bigint      NOT NULL,
    -- The bex subject the steer runs as: the member who queued the message,
    -- or the last one who edited it.
    author         text        NOT NULL CHECK (char_length(author) BETWEEN 1 AND 200),
    prompt         text        NOT NULL CHECK (char_length(prompt) BETWEEN 1 AND 100000),
    -- The rest of the validated steer request (attachment ids, skills, secret
    -- names, egress hosts, output schema); never an inline secret value.
    request        jsonb       NOT NULL DEFAULT '{}'::jsonb,
    -- A client's key for a retried send, unique per session.
    idempotency_key text       CHECK (idempotency_key IS NULL OR char_length(idempotency_key) BETWEEN 1 AND 128),
    -- Digest of what the key was first used for; the same key with another
    -- request is refused.
    request_hash   text        NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    claimed_until  timestamptz,
    -- When the dispatcher last tried it: the claim goes to the session tried
    -- longest ago, so sessions whose messages keep being blocked cannot hold
    -- every claim slot against the others.
    last_attempt_at timestamptz,
    -- Why the head could not start a turn on the last attempt (a spend
    -- ceiling, the live-sandbox limit); cleared when it does.
    blocked_reason text        NOT NULL DEFAULT '' CHECK (char_length(blocked_reason) <= 500),
    delivered_turn integer,
    delivered_at   timestamptz,
    dropped_reason text
);

-- A session's pending messages in queue order.
CREATE INDEX agent_session_queued_messages_pending_idx
    ON agent_session_queued_messages (session_id, position)
    WHERE delivered_at IS NULL AND dropped_reason IS NULL;

CREATE UNIQUE INDEX agent_session_queued_messages_idempotency_idx
    ON agent_session_queued_messages (session_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
