-- Reverse w9/m345: drop the pipeline_minutes_exhausted facts and the
-- SetCacheProfile toggle column.
DELETE FROM service_event_facts WHERE fact_type = 'pipeline_minutes_exhausted';
ALTER TABLE service_event_facts DROP CONSTRAINT service_event_facts_fact_type_check;
ALTER TABLE service_event_facts ADD CONSTRAINT service_event_facts_fact_type_check
    CHECK (fact_type IN (
        'image_pull_failed', 'service_suspended', 'service_resumed', 'service_hibernated', 'service_woken',
        'server_failed', 'server_hardware_failure', 'server_available', 'branch_changed', 'branch_deleted',
        'commit_ignored', 'autoscaling_started', 'autoscaling_ended', 'build_started', 'build_ended',
        'pre_deploy_started', 'pre_deploy_ended', 'job_run_ended', 'cron_job_run_started', 'cron_job_run_ended',
        'service_disk_usage_high', 'service_disk_usage_recovered'
    ));
ALTER TABLE audit_events
  DROP COLUMN IF EXISTS edge_cache_enabled_to;
