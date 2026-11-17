-- The previous closed sets cannot hold these facts, so they go, as 0188's down
-- path drops build_source facts. Only an automation-fired session carries the
-- 'automation' surface; anything else that does reads as the honest 'unknown'.
DELETE FROM product_activity_events WHERE resource_type IN ('agent_session', 'agent_automation', 'sandbox');
UPDATE product_activity_events SET surface = 'unknown' WHERE surface = 'automation';
ALTER TABLE product_activity_events DROP CONSTRAINT product_activity_events_surface_check,
    ADD CONSTRAINT product_activity_events_surface_check CHECK (surface IN ('dashboard', 'cli', 'mcp', 'api', 'blueprint', 'unknown'));
ALTER TABLE product_activity_events DROP CONSTRAINT product_activity_events_resource_type_check,
    ADD CONSTRAINT product_activity_events_resource_type_check CHECK (resource_type IN ('workspace', 'web_service', 'static_site', 'private_service', 'background_worker', 'cron_job', 'postgres', 'keyvalue', 'domain', 'build_source'));
