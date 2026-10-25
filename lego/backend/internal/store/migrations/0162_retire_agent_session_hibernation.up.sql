-- ADR059 retired (2026-10-25): agent-session hibernation (ADR059, migration 0073) is
-- retired. An idle session's sandbox is now terminated after the idle TTL and
-- the next message continues the conversation in a fresh sandbox on the same
-- branch (ADR047 D3), so no row may stay in a hibernation phase or carry a
-- snapshot.
--
-- Legacy rows land in the terminal-but-steerable phase a completed-and-reaped
-- session already uses:
--   * hibernating / hibernated rows, and resuming rows left by a rehydrate
--     (a snapshot still on the row, or no sandbox ever adopted — the bex-api
--     restart orphan ags-dbdh96fsf6rc738q771g), become `failed` when they carry
--     a failure reason or their latest accepted turn never ran, and
--     `completed` otherwise;
--   * sandbox_id is cleared, so inventory reconcile reclaims any pod still
--     running for them (a terminal session naming no sandbox owns none).
-- updated_at is deliberately untouched: the rows did not newly finish, and the
-- push worker keys on it.
-- One statement, so the classification is read before either write lands.
-- The accepted turn a rehydrate never ran is settled, as a failed dispatch
-- settles it, so the transcript reads it as finished-incomplete.
WITH retired AS (
    SELECT s.id,
           s.turns,
           (s.failure_reason <> ''
            OR EXISTS (SELECT 1 FROM agent_session_turns t
                       WHERE t.session_id = s.id AND t.turn = s.turns
                         AND t.completed_at IS NULL)) AS failed
    FROM agent_sessions s
    WHERE s.phase IN ('hibernating', 'hibernated')
       OR (s.phase = 'resuming' AND (s.snapshot_ref <> '' OR s.sandbox_id = ''))
), settled AS (
    UPDATE agent_session_turns t
    SET completed_at = now()
    FROM retired r
    WHERE t.session_id = r.id AND t.turn = r.turns AND t.completed_at IS NULL
)
UPDATE agent_sessions s
SET phase = CASE WHEN r.failed THEN 'failed' ELSE 'completed' END,
    status = CASE WHEN r.failed THEN 'failed' ELSE 'completed' END,
    failure_reason = CASE
        WHEN NOT r.failed THEN ''
        WHEN s.failure_reason <> '' THEN s.failure_reason
        ELSE 'the sandbox restore was interrupted; send a message to continue in a fresh sandbox'
    END,
    failure_reason_code = '',
    sandbox_id = ''
FROM retired r
WHERE s.id = r.id;

DROP INDEX IF EXISTS agent_sessions_hibernated_idx;

ALTER TABLE agent_sessions DROP CONSTRAINT agent_sessions_phase_check;
ALTER TABLE agent_sessions ADD CONSTRAINT agent_sessions_phase_check
    CHECK (phase IN ('creating', 'running', 'resuming', 'redispatching',
                     'completed', 'failed', 'canceling', 'canceled'));

ALTER TABLE agent_sessions
    DROP COLUMN pinned,
    DROP COLUMN snapshot_ref,
    DROP COLUMN snapshot_bytes,
    DROP COLUMN snapshot_sha,
    DROP COLUMN hibernated_at,
    DROP COLUMN retain_until;
