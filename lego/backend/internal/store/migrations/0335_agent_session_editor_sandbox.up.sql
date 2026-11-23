-- w6/m564 (ADR054 D8): the sandbox `resume` started, or kept, for a person's
-- editor. While it equals sandbox_id the session's sandbox is an editor's: it
-- closes on a person's idle timeout whoever started the session, its close
-- reads the checkout in the sandbox, and that close may keep its disk again
-- for a turn whose kept disk it booted from. '' on every existing row.
ALTER TABLE agent_sessions
    ADD COLUMN editor_sandbox_id text NOT NULL DEFAULT '';
