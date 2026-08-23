-- w6/m47 t002: a free-tier App that auto-hibernates on its idle timeout used to
-- record the SAME service_suspended fact a user clicking Suspend produces, so
-- the Events tab, webhooks, and push notifications could not tell a routine
-- sleep cycle from a real, human-initiated suspension. Auto-sleep gets its own
-- pair of facts; service_suspended/service_resumed stay exclusively user-driven,
-- matching apps.suspenders()'s documented "user is the only suspender" invariant.
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

-- Phase alone conflates the two: an auto-hibernated App and a user-suspended one
-- both observe Hibernated. The checkpoint therefore has to remember WHY it went
-- down, or the wake edge cannot pick between service_resumed and service_woken.
-- FALSE is the correct backfill: every pre-migration row predates the split, and
-- treating an unknown baseline as user-driven would relabel real sleeps.
ALTER TABLE service_event_checkpoints
    ADD COLUMN IF NOT EXISTS suspended BOOLEAN NOT NULL DEFAULT FALSE;
