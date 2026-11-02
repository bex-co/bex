ALTER TABLE deploys
    DROP COLUMN end_timed_out_seconds,
    DROP COLUMN end_oom_killed,
    DROP COLUMN end_exit_code;
