-- ADR059 D2/D3 / w2/m68: the Hibernated tier. An idle agent-session sandbox is
-- reclaimed to an ADR050-encrypted filesystem snapshot in object storage
-- (option B, per the D7 spike) instead of Terminated-and-gone, and rehydrates on
-- the next connect/turn. These columns make the workspace's durable snapshot +
-- retention + the opt-in pinned never-expire tier first-class so list/get can
-- project state and storage cost without touching the (now absent) sandbox.
--
--   pinned         opt-in never-expire (D5): removes only the retention delete
--                  edge; still billed for storage + counted against a pin quota.
--   snapshot_ref   object-storage key (per-workspace prefix, never a registry);
--                  empty => no durable snapshot (live or pre-hibernation).
--   snapshot_bytes snapshot size, the storage-metering + quota dimension (D6).
--   snapshot_sha   integrity digest; a mismatch on restore => clean re-clone.
--   hibernated_at  when the row entered Hibernated (NULL while Active).
--   retain_until   retention deadline: hibernated_at + window (7d default,
--                  extended when the git tree is dirty). NULL while Active or
--                  pinned; the retention sweep deletes an unpinned row past it.
ALTER TABLE agent_sessions
    ADD COLUMN pinned         boolean     NOT NULL DEFAULT false,
    ADD COLUMN snapshot_ref   text        NOT NULL DEFAULT '',
    ADD COLUMN snapshot_bytes bigint      NOT NULL DEFAULT 0,
    ADD COLUMN snapshot_sha   text        NOT NULL DEFAULT '',
    ADD COLUMN hibernated_at  timestamptz,
    ADD COLUMN retain_until   timestamptz;

-- 'hibernating' is the transient reclaim window (snapshot upload in flight);
-- 'hibernated' is the durable pod-less state holding a snapshot_ref. Both are
-- non-terminal in the product sense (a hibernated workspace rehydrates), but the
-- Completer's live-turn set (activePhases) excludes them — a hibernated session
-- runs no turn until Resume/Steer transitions it back to a live phase.
ALTER TABLE agent_sessions DROP CONSTRAINT agent_sessions_phase_check;
ALTER TABLE agent_sessions ADD CONSTRAINT agent_sessions_phase_check
    CHECK (phase IN ('creating', 'running', 'resuming', 'redispatching',
                     'hibernating', 'hibernated',
                     'completed', 'failed', 'canceling', 'canceled'));

-- The retention sweep scans unpinned hibernated rows whose deadline has passed;
-- the pin-quota check counts a workspace's pinned+hibernated rows. A partial
-- index on the durable-snapshot working set keeps both cheap regardless of how
-- much terminal history accumulates.
CREATE INDEX agent_sessions_hibernated_idx
    ON agent_sessions (retain_until)
    WHERE phase = 'hibernated' AND snapshot_ref <> '';
