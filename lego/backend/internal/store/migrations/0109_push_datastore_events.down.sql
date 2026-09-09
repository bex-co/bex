-- Rows the narrower 0070 constraints reject must go before they can be
-- restored; per-device rows cascade from the logical inbox rows.
DELETE FROM push_notifications
WHERE resource_kind IN ('database', 'keyValue');

ALTER TABLE push_notifications DROP CONSTRAINT push_notifications_event_type_check;
ALTER TABLE push_notifications ADD CONSTRAINT push_notifications_event_type_check
    CHECK (event_type IN (
        'deploy_started', 'deploy_succeeded', 'deploy_failed',
        'server_failed', 'server_available',
        'service_suspended', 'service_resumed',
        'cron_failed',
        'agent_pr_ready', 'agent_failed', 'agent_needs_decision'
    ));

ALTER TABLE push_notifications DROP CONSTRAINT push_notifications_resource_check;
ALTER TABLE push_notifications ADD CONSTRAINT push_notifications_resource_check
    CHECK (
        (resource_kind = 'service' AND resource_id ~ '^srv-[0-9a-v]{20}$')
        OR (resource_kind = 'agentSession' AND resource_id ~ '^ags-[0-9a-v]{20}$')
    );

ALTER TABLE push_notifications DROP CONSTRAINT push_notifications_deep_link_check;
ALTER TABLE push_notifications ADD CONSTRAINT push_notifications_deep_link_check
    CHECK (
        (resource_kind = 'service' AND deep_link = '/services/' || resource_id)
        OR (resource_kind = 'agentSession' AND deep_link = '/sessions/' || resource_id)
    );
