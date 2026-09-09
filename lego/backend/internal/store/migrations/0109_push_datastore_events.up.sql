-- w3/m82 t005: managed datastores join the push inbox, so 0070's three CHECKs
-- are rewritten as their union with a third resource family.
--
-- Availability (postgres_unavailable / key_value_unhealthy and their recovery
-- siblings) is the Critical/Important pair server_failed and server_available
-- already model. Only the FAILURE half of backup, restore, and major upgrade
-- is push-worthy: a nightly backup that worked is not news, and stays in the
-- Events feed and the webhook vocabulary. Every name here is additive and
-- opt-in — the member default policy in notification_settings is unchanged.
--
-- Postgres and Key Value are separate resource kinds rather than one shared
-- 'datastore' because their ids and deep links genuinely differ (dpg- vs red-,
-- /databases/ vs /key-values/), and pairing the three columns is the whole
-- point of these constraints.

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
