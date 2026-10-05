-- Intentionally retain the added columns and the relaxed reason-code check on
-- rollback. Older binaries never write exit_code or run_id (both default), and
-- narrowing the check would reject retained cron_job_run_ended facts that
-- already carry a failure reason. No event rows are rewritten or deleted.
SELECT 1;
