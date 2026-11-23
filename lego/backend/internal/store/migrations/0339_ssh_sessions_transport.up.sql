-- w6/m564 t009: record how each audited gateway session reached its target.
--
-- Since ADR077 round 17 the agent attach (an agent session's conversation
-- stream, which the dashboard opens on every session page) writes an
-- ssh_sessions row keyed by the session id, as an SSH login does. The agent
-- idle close (ADR054 D6, ADR059) read every such row as an editor: a page view
-- moved the idle clock, and a page left open on a sandbox started for an
-- editor (ADR054 D8), whose live stream never ends, held it for the 4-hour
-- SSH cap. Only an SSH login is an editor; the close now reads those rows.
--
-- Rows written before this column, and by a gateway replica of the previous
-- release while the deploy rolls, are recorded as SSH. The attach rows among
-- them stop counting at the 4-hour cap like any row.
ALTER TABLE ssh_sessions
  ADD COLUMN IF NOT EXISTS transport text NOT NULL DEFAULT 'ssh'
    CONSTRAINT ssh_sessions_transport_check
    CHECK (transport IN ('ssh', 'web_shell', 'agent_attach'));
