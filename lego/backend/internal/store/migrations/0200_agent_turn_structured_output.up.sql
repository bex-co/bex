-- ADR094 D5 (w21/m259): a turn can carry a JSON Schema its result must satisfy.
-- output_schema is the validated schema (NULL = free-form turn), output_required
-- says a turn that ends without a final valid result fails
-- STRUCTURED_OUTPUT_MISSING, and result holds the agent's last valid
-- submission. result_final marks the one the agent declared final: only that
-- value is the turn's outcome.result; a non-final result is a partial value a
-- poller can read while the turn runs.
ALTER TABLE agent_session_turns
    ADD COLUMN output_schema   jsonb,
    ADD COLUMN output_required boolean NOT NULL DEFAULT false,
    ADD COLUMN result          jsonb,
    ADD COLUMN result_final    boolean NOT NULL DEFAULT false;
