-- w4/m114: a failed cron run's cron_job_run_ended fact records why it failed
-- (Render's "Cron Job Run Ended" reason object) and the run's public crr- id.
ALTER TABLE service_event_facts ADD COLUMN IF NOT EXISTS exit_code INTEGER;
ALTER TABLE service_event_facts ADD COLUMN IF NOT EXISTS run_id TEXT NOT NULL DEFAULT '';
ALTER TABLE service_event_facts DROP CONSTRAINT service_event_facts_reason_code_check;
ALTER TABLE service_event_facts ADD CONSTRAINT service_event_facts_reason_code_check
    CHECK (reason_code IN (
        '',
        'image_pull_backoff',
        'readiness_failed',
        'root_directory',
        'build_filter',
        'skip_phrase',
        'superseded',
        'non_zero_exit',
        'oom_killed',
        'evicted',
        'timed_out'
    ));
