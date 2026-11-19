-- Reverse w4/m504: drop zero_downtime_redeploy_* facts and restore the prior
-- CHECK constraints.
DELETE FROM service_event_facts
WHERE fact_type IN ('zero_downtime_redeploy_started', 'zero_downtime_redeploy_ended')
   OR reason_code IN ('node_replacement', 'node_consolidation');

ALTER TABLE service_event_facts DROP CONSTRAINT service_event_facts_fact_type_check;
ALTER TABLE service_event_facts ADD CONSTRAINT service_event_facts_fact_type_check
    CHECK (fact_type IN (
        'image_pull_failed', 'service_suspended', 'service_resumed', 'service_hibernated', 'service_woken',
        'server_failed', 'server_hardware_failure', 'server_available', 'branch_changed', 'branch_deleted',
        'commit_ignored', 'autoscaling_started', 'autoscaling_ended', 'build_started', 'build_ended',
        'pre_deploy_started', 'pre_deploy_ended', 'job_run_ended', 'cron_job_run_started', 'cron_job_run_ended',
        'service_disk_usage_high', 'service_disk_usage_recovered', 'pipeline_minutes_exhausted'
    ));

ALTER TABLE service_event_facts DROP CONSTRAINT service_event_facts_reason_code_check;
ALTER TABLE service_event_facts ADD CONSTRAINT service_event_facts_reason_code_check
    CHECK (reason_code IN (
        '', 'image_pull_backoff', 'readiness_failed', 'root_directory', 'build_filter',
        'skip_phrase', 'superseded', 'non_zero_exit', 'oom_killed', 'evicted',
        'timed_out', 'early_exit'
    ));
