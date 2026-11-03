DELETE FROM alert_email_streams WHERE event IN ('disk_usage_high', 'disk_usage_recovered');
ALTER TABLE alert_email_streams DROP CONSTRAINT alert_email_streams_event_check;
ALTER TABLE alert_email_streams ADD CONSTRAINT alert_email_streams_event_check CHECK (event IN (
    'server_failed', 'cron_failed', 'image_pull_failed', 'blueprint_sync_failed', 'service_suspended'));
DROP TABLE IF EXISTS service_disk_usage_state;
DROP TABLE IF EXISTS service_event_disk_usage;
DELETE FROM service_event_facts WHERE fact_type IN ('service_disk_usage_high', 'service_disk_usage_recovered');
ALTER TABLE service_event_facts DROP CONSTRAINT service_event_facts_fact_type_check;
ALTER TABLE service_event_facts ADD CONSTRAINT service_event_facts_fact_type_check
    CHECK (fact_type IN (
        'image_pull_failed', 'service_suspended', 'service_resumed', 'service_hibernated', 'service_woken',
        'server_failed', 'server_hardware_failure', 'server_available', 'branch_changed', 'branch_deleted',
        'commit_ignored', 'autoscaling_started', 'autoscaling_ended', 'build_started', 'build_ended',
        'pre_deploy_started', 'pre_deploy_ended', 'job_run_ended', 'cron_job_run_started', 'cron_job_run_ended'
    ));
