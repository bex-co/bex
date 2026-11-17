-- w3/m441 (ADR113 D7 server facts): agent sessions, automations and sandboxes
-- record which surface created them, as services and datastores already do, so
-- "does the CLI start real work?" is answered on the server rather than by
-- collecting more on the client.
--
-- 'automation' joins the closed surface set for a session an automation starts:
-- like a Blueprint apply (0117), the declared automation wins over whichever
-- transport triggered the run, and the scheduler is not a request at all.
--
-- Both CHECKs are re-added NOT VALID and validated separately (0117, 0188), so
-- the scan of the append-only event log does not hold a lock that blocks its
-- writers.
ALTER TABLE product_activity_events DROP CONSTRAINT product_activity_events_resource_type_check,
    ADD CONSTRAINT product_activity_events_resource_type_check CHECK (resource_type IN ('workspace', 'web_service', 'static_site', 'private_service', 'background_worker', 'cron_job', 'postgres', 'keyvalue', 'domain', 'build_source', 'agent_session', 'agent_automation', 'sandbox')) NOT VALID;
ALTER TABLE product_activity_events VALIDATE CONSTRAINT product_activity_events_resource_type_check;
ALTER TABLE product_activity_events DROP CONSTRAINT product_activity_events_surface_check,
    ADD CONSTRAINT product_activity_events_surface_check CHECK (surface IN ('dashboard', 'cli', 'mcp', 'api', 'blueprint', 'automation', 'unknown')) NOT VALID;
ALTER TABLE product_activity_events VALIDATE CONSTRAINT product_activity_events_surface_check;
