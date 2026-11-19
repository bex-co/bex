-- w4/m510: Render's artifact_source_changed and artifact_fetch_failed service
-- events for Build Sources (ADR097).
--
-- audit_events stores the previous and new build source id on apps.LinkBuildSource
-- rows so the service events feed and webhooks can project artifact_source_changed
-- with Render's fromBuildSourceId/toBuildSourceId (absent side NULL/omitted) while
-- keeping pre-migration rows (both NULL, whose sibling feed_only row already
-- appeared as source_changed) out of the feed.
ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS build_source_from text,
  ADD COLUMN IF NOT EXISTS build_source_to text;

-- service_event_facts gains artifact_fetch_failed plus the build_id and pull
-- failure message Render requires on Artifact Fetch Failed.
-- The CHECK constraint only gains a value, so NOT VALID avoids scanning this
-- append-only table under an access-exclusive lock.
ALTER TABLE service_event_facts DROP CONSTRAINT service_event_facts_fact_type_check,
    ADD CONSTRAINT service_event_facts_fact_type_check
    CHECK (fact_type IN (
        'image_pull_failed',
        'artifact_fetch_failed',
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

ALTER TABLE service_event_facts
  ADD COLUMN IF NOT EXISTS build_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS message text NOT NULL DEFAULT '';
