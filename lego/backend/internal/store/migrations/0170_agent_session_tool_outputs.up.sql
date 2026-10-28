-- ADR051 amendment 2026-10-27 (w10/m18): full tool outputs out of line. The
-- durable transcript keeps a clipped tool output inline as head + tail (8,000
-- characters around `[bex: N chars clipped]`); the complete text lives here,
-- one row per clipped field, read only when a user asks for it.
--
-- field: 'output', 'errorText', or 'terminal:<terminalId>' ('terminal' when the
-- driver sent no id). output is capped at 1 MiB per row by the writers (the
-- cut is marked in the text); a session's rows together are capped by
-- store.MaxAgentSessionToolOutputBytes, checked in the INSERT itself.
--
-- Both writers insert idempotently (ON CONFLICT DO NOTHING): the gateway tee
-- (SELECT, INSERT only) and the Completer harvest. Rows cascade with their
-- session, like the transcript.
CREATE TABLE agent_session_tool_outputs (
    session_id   text        NOT NULL REFERENCES agent_sessions (id) ON DELETE CASCADE,
    turn         int         NOT NULL,
    tool_call_id text        NOT NULL CHECK (length(tool_call_id) BETWEEN 1 AND 512),
    field        text        NOT NULL CHECK (length(field) BETWEEN 1 AND 600),
    output       text        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, turn, tool_call_id, field)
);
