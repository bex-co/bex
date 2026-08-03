-- ADR047 D9 / w3/m43: durable per-session UI-message-stream transcript.
-- The isolated gateway tees every part of an agent session's Vercel-AI-SDK
-- UI-message stream (the verbatim `data:` payloads the in-sandbox driver emits)
-- into this table as it reverse-proxies. It is the single history source: the
-- stream endpoint's replay mode reads it for reattach AND for terminal-session
-- history, so the transcript must outlive the sandbox (which the driver's own
-- in-memory history does not).
--
-- seq is the driver's emission ordinal on the /stream it proxies (0-based). The
-- driver deterministically replays its full history in order to every attacher,
-- so the same part gets the same seq on every gateway replica; the primary key
-- + ON CONFLICT DO NOTHING append therefore makes the tee idempotent across
-- replicas and re-attaches (no duplication, no gap).
--
-- part is text, NOT jsonb, on purpose: jsonb canonicalizes (reorders keys, adds
-- whitespace) on write, which would corrupt the verbatim-forward guarantee
-- (ADR047 D3) — replay must return the exact bytes the driver emitted so a
-- Vercel AI SDK client sees a byte-identical UI-message stream. text preserves
-- them.
CREATE TABLE agent_session_transcripts (
    session_id text        NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    seq        bigint      NOT NULL,
    turn       int         NOT NULL DEFAULT 0,
    part       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, seq)
);

-- Retention prune scans by wall-clock age across sessions (the daily sweep,
-- BEX_AUDIT_RETENTION_DAYS lineage); ordered replay uses the primary key.
CREATE INDEX agent_session_transcripts_created_idx
    ON agent_session_transcripts (created_at);
