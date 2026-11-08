-- w2/m278 (ADR014 D4, ADR042 D5): idle suspension for Render sandboxes.
--
-- Render suspends a sandbox after idleTimeoutSeconds with no command execution
-- or file transfer. bex-api brokers every exec and file transfer, so it is the
-- activity signal: each one bumps last_activity_at (and a running stream
-- re-bumps it every minute, so a long command is never idle).
--
-- suspend_claimed_at is the idle sweeper's compare-and-set: a replica claims a
-- sandbox only while last_activity_at is past its idle window, then pauses it.
-- An exec that touches the row after a claim waits for that pause and resumes
-- the sandbox, so a command never runs into a sandbox that is being paused.
CREATE TABLE sandbox_activity (
    workspace_id       text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    sandbox_id         text        NOT NULL,
    last_activity_at   timestamptz NOT NULL,
    suspend_claimed_at timestamptz,
    PRIMARY KEY (workspace_id, sandbox_id)
);
