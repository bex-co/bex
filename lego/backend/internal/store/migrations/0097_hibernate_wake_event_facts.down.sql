DELETE FROM service_event_facts WHERE fact_type IN (
    'service_hibernated',
    'service_woken'
);

ALTER TABLE service_event_facts DROP CONSTRAINT service_event_facts_fact_type_check;
ALTER TABLE service_event_facts ADD CONSTRAINT service_event_facts_fact_type_check
    CHECK (fact_type IN (
        'image_pull_failed',
        'service_suspended',
        'service_resumed',
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

ALTER TABLE service_event_checkpoints DROP COLUMN IF EXISTS suspended;
