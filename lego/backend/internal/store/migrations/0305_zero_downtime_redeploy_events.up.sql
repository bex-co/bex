-- w4/m504: Render's zero_downtime_redeploy_started and
-- zero_downtime_redeploy_ended service events, produced when drain-guard hands
-- a single-instance App off a node about to be replaced or consolidated.
--
-- Both lists only gain values, so no existing row can fail them: NOT VALID
-- avoids scanning this append-only table under an access-exclusive lock.
ALTER TABLE service_event_facts DROP CONSTRAINT service_event_facts_fact_type_check,
    ADD CONSTRAINT service_event_facts_fact_type_check
    CHECK (fact_type IN (
        'image_pull_failed',
        'service_suspended',
        'service_resumed',
        'service_hibernated',
        'service_woken',
        'server_failed',
        'server_hardware_failure',
        'server_available',
        'branch_changed',
        'branch_deleted',
        'commit_ignored',
        'autoscaling_started',
        'autoscaling_ended',
        'build_started',
        'build_ended',
        'pre_deploy_started',
        'pre_deploy_ended',
        'job_run_ended',
        'cron_job_run_started',
        'cron_job_run_ended',
        'service_disk_usage_high',
        'service_disk_usage_recovered',
        'pipeline_minutes_exhausted',
        'zero_downtime_redeploy_started',
        'zero_downtime_redeploy_ended'
    )) NOT VALID;

ALTER TABLE service_event_facts DROP CONSTRAINT service_event_facts_reason_code_check,
    ADD CONSTRAINT service_event_facts_reason_code_check
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
        'early_exit',
        'node_replacement',
        'node_consolidation'
    )) NOT VALID;
