-- Rows the narrower 0063 constraints reject must go before they can be
-- restored; per-device rows cascade from the logical inbox rows.
DELETE FROM push_notifications
WHERE event_type IN ('server_available', 'service_suspended', 'service_resumed',
                     'agent_pr_ready', 'agent_failed', 'agent_needs_decision')
   OR resource_kind <> 'service';

ALTER TABLE push_notifications DROP CONSTRAINT push_notifications_event_type_check;
ALTER TABLE push_notifications ADD CONSTRAINT push_notifications_event_type_check
    CHECK (event_type IN (
        'deploy_started', 'deploy_succeeded', 'deploy_failed',
        'server_failed', 'cron_failed'
    ));

ALTER TABLE push_notifications DROP CONSTRAINT push_notifications_resource_check;
ALTER TABLE push_notifications ADD CONSTRAINT push_notifications_resource_check
    CHECK (resource_kind = 'service' AND resource_id ~ '^srv-[0-9a-v]{20}$');

ALTER TABLE push_notifications DROP CONSTRAINT push_notifications_deep_link_check;
ALTER TABLE push_notifications ADD CONSTRAINT push_notifications_deep_link_check
    CHECK (deep_link = '/services/' || resource_id);
