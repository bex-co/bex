-- Rows the narrower 0109 constraints reject must go before they can be
-- restored; per-device rows cascade from the logical inbox rows.
DELETE FROM push_notifications
WHERE resource_kind = 'agentAutomation';

ALTER TABLE push_notifications DROP CONSTRAINT push_notifications_event_type_check;
ALTER TABLE push_notifications ADD CONSTRAINT push_notifications_event_type_check
    CHECK (event_type IN (
        'deploy_started', 'deploy_succeeded', 'deploy_failed',
        'server_failed', 'server_available',
        'service_suspended', 'service_resumed',
        'cron_failed',
        'agent_pr_ready', 'agent_failed', 'agent_needs_decision',
        'postgres_unavailable', 'postgres_available',
        'key_value_unhealthy', 'key_value_available',
        'postgres_backup_failed', 'postgres_restore_failed', 'postgres_upgrade_failed'
    ));

ALTER TABLE push_notifications DROP CONSTRAINT push_notifications_resource_check;
ALTER TABLE push_notifications ADD CONSTRAINT push_notifications_resource_check
    CHECK (
        (resource_kind = 'service' AND resource_id ~ '^srv-[0-9a-v]{20}$')
        OR (resource_kind = 'agentSession' AND resource_id ~ '^ags-[0-9a-v]{20}$')
        OR (resource_kind = 'database' AND resource_id ~ '^dpg-[0-9a-v]{20}$')
        OR (resource_kind = 'keyValue' AND resource_id ~ '^red-[0-9a-v]{20}$')
    );

ALTER TABLE push_notifications DROP CONSTRAINT push_notifications_deep_link_check;
ALTER TABLE push_notifications ADD CONSTRAINT push_notifications_deep_link_check
    CHECK (
        (resource_kind = 'service' AND deep_link = '/services/' || resource_id)
        OR (resource_kind = 'agentSession' AND deep_link = '/sessions/' || resource_id)
        OR (resource_kind = 'database' AND deep_link = '/databases/' || resource_id)
        OR (resource_kind = 'keyValue' AND deep_link = '/key-values/' || resource_id)
    );

DROP INDEX agent_automation_runs_notify_idx;
ALTER TABLE agent_automation_runs DROP COLUMN notified_at;

DROP INDEX agent_automations_pause_unnotified_idx;
DROP INDEX agent_automations_pause_notify_idx;
ALTER TABLE agent_automations
    DROP COLUMN pause_notified_at,
    DROP COLUMN paused_at,
    DROP COLUMN notify_failure_streak,
    DROP COLUMN notify_email;
