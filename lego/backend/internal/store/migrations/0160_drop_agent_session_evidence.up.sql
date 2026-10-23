-- ADR090: the driver-derived evidence digest (added by 0066) is
-- gone. It had no renderer since w5/m65 and duplicated the ADR051 transcript
-- lossily; the agent's own PR description now carries what changed and how it
-- was verified.
ALTER TABLE agent_sessions
    DROP COLUMN evidence;
