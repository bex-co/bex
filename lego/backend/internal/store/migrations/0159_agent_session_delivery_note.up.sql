-- ADR090: the coding agent now commits, pushes and opens its own
-- draft PR. When a turn ends with work the agent did not deliver (a dirty tree,
-- unpushed commits, or a pushed branch with no PR) the session still completes,
-- and this column carries the readable note saying so. '' means nothing to
-- report: the agent delivered, or the turn changed nothing.
ALTER TABLE agent_sessions
    ADD COLUMN delivery_note text NOT NULL DEFAULT '';
