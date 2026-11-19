-- A session waits for a sandbox machine when the sandbox pool is full, and a
-- start that still fails names its cause (w10/m497, ADR047 § w10/m497).
--
-- sandbox_wait_since is when the current turn's dispatch began waiting for room
-- on the pool; NULL when it is not waiting. failure_cause is the start
-- failure's cause beside failure_reason_code SANDBOX_START_FAILED: pool_full,
-- image_pull, runtime, quota or unknown; '' otherwise.
ALTER TABLE agent_sessions
    ADD COLUMN sandbox_wait_since timestamptz,
    ADD COLUMN failure_cause text NOT NULL DEFAULT '';
