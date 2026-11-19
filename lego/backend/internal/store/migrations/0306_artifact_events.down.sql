-- Reverse w4/m510: drop artifact_fetch_failed facts/columns and the build-source
-- link columns on audit_events, restoring the prior fact_type CHECK constraint.
DELETE FROM service_event_facts
WHERE fact_type = 'artifact_fetch_failed';

ALTER TABLE service_event_facts
  DROP COLUMN IF EXISTS message,
  DROP COLUMN IF EXISTS build_id;

ALTER TABLE service_event_facts DROP CONSTRAINT service_event_facts_fact_type_check;
ALTER TABLE service_event_facts ADD CONSTRAINT service_event_facts_fact_type_check
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
    ));

ALTER TABLE audit_events
  DROP COLUMN IF EXISTS build_source_to,
  DROP COLUMN IF EXISTS build_source_from;
