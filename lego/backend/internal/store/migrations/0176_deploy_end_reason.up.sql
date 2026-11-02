-- w8/m146: the structured cause of a failed rollout, from the operator's
-- status.rolloutFailure, so deploy_ended can carry Render's reason.failure
-- (nonZeroExit / oomKilled / timedOutSeconds) without reading failure_reason
-- text. Closed facts only: an exit code, a flag and a duration.
ALTER TABLE deploys
    ADD COLUMN end_exit_code INTEGER CHECK (end_exit_code IS NULL OR end_exit_code <> 0),
    ADD COLUMN end_oom_killed BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN end_timed_out_seconds INTEGER NOT NULL DEFAULT 0 CHECK (end_timed_out_seconds >= 0);
