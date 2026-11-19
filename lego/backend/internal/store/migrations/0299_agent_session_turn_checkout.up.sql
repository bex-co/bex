-- w10/m491, ADR047 D12: a turn's disk is kept for the next message whatever
-- ended the turn, when its checkout holds work it did not deliver.
--
-- checkout_dirty and unpushed_commits are what the driver measured in the
-- turn's checkout when the turn ended (uncommitted changes, and local commits
-- not on the remote session branch), or what bex-api read in the sandbox when
-- it decided whether to keep the disk. NULL is unknown: a session with no
-- repository, a setup failure, a status from before this migration.
--
-- retention_cause is what ended the sandbox whose disk was kept or lost:
-- stopped (a person stopped the turn), failed (the turn failed), replaced (the
-- next message replaced the sandbox) or idle_closed (the idle grace closed
-- it); '' when nothing was decided for the turn.
--
-- The check is added NOT VALID: every existing row holds the default, and
-- skipping the validation scan keeps the lock brief. New and updated rows are
-- still checked.
ALTER TABLE agent_session_turns
    ADD COLUMN checkout_dirty boolean,
    ADD COLUMN unpushed_commits integer,
    ADD COLUMN retention_cause text NOT NULL DEFAULT '';

ALTER TABLE agent_session_turns
    ADD CONSTRAINT agent_session_turns_checkout_check
        CHECK (unpushed_commits IS NULL OR unpushed_commits >= 0) NOT VALID,
    ADD CONSTRAINT agent_session_turns_retention_cause_check
        CHECK (retention_cause IN ('', 'stopped', 'failed', 'replaced', 'idle_closed')) NOT VALID;
