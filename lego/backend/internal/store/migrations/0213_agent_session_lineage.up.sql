-- ADR102: who spawned whom. A child session records its parent, the top of
-- its tree and its distance from that top.
--
-- parent_session_id has a same-table foreign key ON DELETE SET NULL: a parent
-- can only be deleted once finished, and the children it spawned are
-- independent work that must survive it (they become top-level rows in the
-- list). Deleting a parent never deletes a child. root_session_id has no
-- foreign key for the same reason, so a tree keeps its root id (and the
-- subtree query keeps working) after the root is deleted. A top-level session
-- has both NULL and depth 0. The fan-out and depth limits are enforced in the
-- store's insert transaction under a row lock on the parent; the CHECK here
-- is the schema-level backstop for the depth limit.
ALTER TABLE agent_sessions
    ADD COLUMN parent_session_id text REFERENCES agent_sessions (id) ON DELETE SET NULL,
    ADD COLUMN root_session_id   text,
    ADD COLUMN depth             integer NOT NULL DEFAULT 0 CHECK (depth BETWEEN 0 AND 3);

-- Children listing, the live-children count and the cascade walk.
CREATE INDEX agent_sessions_parent_idx
    ON agent_sessions (parent_session_id, created_at, id)
    WHERE parent_session_id IS NOT NULL;

-- Subtree of a root.
CREATE INDEX agent_sessions_root_idx
    ON agent_sessions (root_session_id, depth, created_at, id)
    WHERE root_session_id IS NOT NULL;
