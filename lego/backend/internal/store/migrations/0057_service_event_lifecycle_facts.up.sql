-- w7/m66: surface the build / pre-deploy / one-off-job lifecycle event types
-- Render emits as distinct timeline entries — build_started/build_ended,
-- pre_deploy_started/pre_deploy_ended, job_run_ended — plus branch_deleted.
-- They ride the same closed, typed service_event_facts path the observed
-- deploy/status facts already use: no JSON, no free-form message, no value
-- column, so a build or step outcome can never carry an env or secret value.

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
        'job_run_ended'
    ));

-- status is the terminal outcome of a lifecycle step — build_ended,
-- pre_deploy_ended, job_run_ended carry succeeded|failed|canceled. Closed and
-- checked like reason_code: a step outcome is never an arbitrary string. Older
-- facts (and the started/observed kinds) read as '' by absence, no rewrite.
ALTER TABLE service_event_facts ADD COLUMN status TEXT NOT NULL DEFAULT ''
    CHECK (status IN ('', 'succeeded', 'failed', 'canceled'));
