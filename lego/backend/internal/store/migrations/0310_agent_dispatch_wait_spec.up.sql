-- A session's wait for a sandbox machine survives its bex-api replica's exit
-- (w10/022, ADR047 § w10/m497).
--
-- wait_spec is the dispatch's input, stored while the dispatch waits for room
-- on the sandbox pool, so another replica's dispatch recovery resumes the wait
-- instead of abandoning it. It holds no secret: secrets, attachments, skills
-- and the repository environment are read again by each dispatch, and the
-- model key is the session's placeholder. NULL when the dispatch does not
-- wait, and cleared when it is abandoned.
ALTER TABLE agent_session_dispatches
    ADD COLUMN wait_spec jsonb;
