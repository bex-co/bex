-- w8/m158: a running service's process that ends on its own with status 0 is
-- Render's "Server Failed" reason earlyExit. Admit that typed reason for a
-- server_failed fact; non_zero_exit and oom_killed are already admitted.
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
        'timed_out',
        'early_exit'
    ));
