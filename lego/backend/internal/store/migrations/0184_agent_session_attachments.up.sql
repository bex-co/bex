-- ADR095: files uploaded into, or shared out of, an agent session. Bytes live
-- in the bex-agent-attachments bucket under object_key; this table holds
-- metadata and is also the purge queue for those objects.
--
-- A row is written 'pending' before its object is stored and becomes 'ready'
-- once it is, so no object can exist without a row the sweep can find. Rows
-- end 'deleted' and stay until the sweep has removed the object.
--
-- workspace_id deliberately has no tenant foreign key (like
-- agent_session_dispatches): a workspace delete retires its rows in the same
-- transaction instead of cascading them away, which would orphan the objects.
-- session_id is nulled when the session row goes; DeleteAgentSession retires
-- the rows first, in its own transaction.
CREATE TABLE agent_session_attachments (
    id                 text        PRIMARY KEY,
    workspace_id       text        NOT NULL,
    session_id         text        REFERENCES agent_sessions(id) ON DELETE SET NULL,
    turn               int         CHECK (turn > 0),
    direction          text        NOT NULL CHECK (direction IN ('input', 'output')),
    name               text        NOT NULL CHECK (length(name) BETWEEN 1 AND 255),
    media_type         text        NOT NULL,
    size_bytes         bigint      NOT NULL CHECK (size_bytes >= 0),
    sha256             text        NOT NULL,
    object_key         text        NOT NULL,
    state              text        NOT NULL CHECK (state IN ('pending', 'ready', 'deleted')),
    created_by         text        NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    expires_at         timestamptz,
    purge_attempted_at timestamptz
);

CREATE INDEX agent_session_attachments_session_idx ON agent_session_attachments (session_id);
CREATE INDEX agent_session_attachments_workspace_idx ON agent_session_attachments (workspace_id);
CREATE INDEX agent_session_attachments_session_order_idx
    ON agent_session_attachments (session_id, created_at, id) WHERE state = 'ready';
CREATE INDEX agent_session_attachments_unbound_idx
    ON agent_session_attachments (workspace_id, created_by) WHERE session_id IS NULL AND state = 'ready';
CREATE INDEX agent_session_attachments_expiry_idx
    ON agent_session_attachments (expires_at) WHERE expires_at IS NOT NULL AND state <> 'deleted';
CREATE INDEX agent_session_attachments_purge_idx
    ON agent_session_attachments (purge_attempted_at NULLS FIRST, created_at) WHERE state = 'deleted';

-- What each turn attached, in submission order: a snapshot of
-- [{id, name, mediaType, size}] taken at binding, which lets the gateway replay
-- a turn without read access to the attachments table.
ALTER TABLE agent_session_turns
    ADD COLUMN attachments jsonb NOT NULL DEFAULT '[]'::jsonb;
