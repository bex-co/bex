-- w8/383: a release whose start command exits 0 during startup records exit
-- code 0, which deploy_ended reports as Render's reason.failure.earlyExit with
-- no nonZeroExit. Dropping a CHECK takes a brief lock and scans nothing.
ALTER TABLE deploys DROP CONSTRAINT deploys_end_exit_code_check;
