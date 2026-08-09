-- w3/m78 (ADR052 gap item 1) + w11/m6: the push inbox grows two event families
-- at once, so the 0063 CHECKs are rewritten as their union.
--
-- Lifecycle (m78): server_available (recovery closing a Critical server_failed
-- page), service_suspended, service_resumed — service-scoped like the existing
-- vocabulary; additive opt-in, the member default policy does not gain them.
--
-- Agent sessions (w11/m6): agent_pr_ready, agent_failed, agent_needs_decision
-- are workspace-scoped bex extensions — their resource is the agent session
-- and the deep link opens /sessions/<ags-…>, which 0063's service-only
-- resource/deep-link CHECKs rejected (the enqueue path shipped before this
-- migration and would fail the constraint at runtime).

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
