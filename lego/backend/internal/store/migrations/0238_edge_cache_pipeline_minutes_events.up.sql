-- w9/m345: Render's edge_cache_enabled / edge_cache_disabled and
-- pipeline_minutes_exhausted events.
--
-- A build the operator refused because the workspace is out of pipeline
-- minutes is recorded, once per refused deploy, as a typed fact. The list
-- only gains a value, so no existing row can fail it: the constraint is
-- re-added NOT VALID, which skips the scan of this append-only table (and the
-- lock it would hold, the whole file being one transaction).
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
        'pipeline_minutes_exhausted'
    )) NOT VALID;

-- An apps.SetCacheProfile row records whether the save turned edge caching on
-- (true) or off (false), as maintenance_mode_to records its toggle. A save that
-- keeps caching on (static <-> all) or off records NULL: Render has no event
-- for it, so the row stays in the workspace audit log and out of the events
-- feed. Rows recorded before this are NULL too.
ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS edge_cache_enabled_to boolean;
