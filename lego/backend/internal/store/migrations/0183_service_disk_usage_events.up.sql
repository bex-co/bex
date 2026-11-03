-- w2/m255: a service disk crossing 80% of its size, and dropping back below,
-- is a Render event (service_disk_usage_high / service_disk_usage_recovered,
-- details {diskId, diskName, mountPath, usagePercent, thresholdPercent}) and a
-- Render email ("A service's persistent disk usage exceeds 80% of the disk's
-- allocated space", and "drops back below 80%").
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
        'service_disk_usage_recovered'
    ));

-- The typed details of a disk-usage fact. A side table keeps the composed
-- event feed's UNION projection unchanged; the feed attaches these by key.
CREATE TABLE service_event_disk_usage (
    source_key        text PRIMARY KEY REFERENCES service_event_facts (source_key) ON DELETE CASCADE,
    disk_id           text NOT NULL,
    disk_name         text NOT NULL,
    mount_path        text NOT NULL,
    usage_percent     double precision NOT NULL CHECK (usage_percent >= 0),
    threshold_percent double precision NOT NULL CHECK (threshold_percent > 0 AND threshold_percent <= 100)
);

-- One row per disk: whether it is over the threshold, and a crossing seen once
-- but not yet confirmed by the next sweep.
CREATE TABLE service_disk_usage_state (
    disk_id       text PRIMARY KEY REFERENCES service_disks (id) ON DELETE CASCADE,
    high          boolean NOT NULL DEFAULT false,
    pending_high  boolean,
    pending_since timestamptz,
    updated_at    timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE alert_email_streams DROP CONSTRAINT alert_email_streams_event_check;
ALTER TABLE alert_email_streams ADD CONSTRAINT alert_email_streams_event_check CHECK (event IN (
    'server_failed', 'cron_failed', 'image_pull_failed', 'blueprint_sync_failed', 'service_suspended',
    'disk_usage_high', 'disk_usage_recovered'));
