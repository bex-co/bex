-- Reverse 0164: hardware failures recorded under the new name become the
-- server_failed they would have been before, then the CHECK narrows.
UPDATE service_event_facts SET fact_type = 'server_failed' WHERE fact_type = 'server_hardware_failure';

ALTER TABLE service_event_facts DROP CONSTRAINT service_event_facts_fact_type_check;
ALTER TABLE service_event_facts ADD CONSTRAINT service_event_facts_fact_type_check
    CHECK (fact_type IN (
        'image_pull_failed',
        'service_suspended',
        'service_resumed',
        'service_hibernated',
        'service_woken',
        'server_failed',
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
        'cron_job_run_ended'
    ));
