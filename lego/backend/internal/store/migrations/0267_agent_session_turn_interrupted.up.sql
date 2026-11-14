-- ADR047 D12: "Stop turn" ends only the running turn. The session
-- finalizes as completed, as for a turn that ended on its own, and the turn
-- records that a person stopped it.
ALTER TABLE agent_session_turns
    ADD COLUMN interrupted boolean NOT NULL DEFAULT false;
