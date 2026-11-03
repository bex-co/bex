-- w2/m254 (ADR097): a Render Build Source is an apps row of type build_source
-- (a carrier the operator builds and never runs). A linked service names its
-- carrier and the carrier build it deploys. A carrier with linked services
-- cannot be deleted out from under them (NO ACTION, not RESTRICT, so a
-- workspace deletion that cascades through both rows in one statement still
-- succeeds).
ALTER TABLE apps
    ADD COLUMN build_source_id text REFERENCES apps (id),
    ADD COLUMN build_source_image text NOT NULL DEFAULT '';
-- A carrier is workspace-level: never grouped into a project or environment
-- (so those listings cannot surface it), and never itself linked.
ALTER TABLE apps ADD CONSTRAINT apps_build_source_shape_check CHECK (
    type <> 'build_source' OR (project_id IS NULL AND environment_id IS NULL AND build_source_id IS NULL));
-- A linked service takes its source from the build source: no repo or image
-- of its own.
ALTER TABLE apps DROP CONSTRAINT apps_check,
    ADD CONSTRAINT apps_check CHECK (repo IS NOT NULL OR image IS NOT NULL OR build_source_id IS NOT NULL);
CREATE INDEX apps_build_source_idx ON apps (build_source_id) WHERE build_source_id IS NOT NULL;

-- A build source's builds and lifecycle are product events like a service's:
-- the deploy and delete triggers record them under the row's type. Re-added
-- NOT VALID and validated separately, so the scan of the append-only event log
-- does not hold a lock that blocks its writers.
ALTER TABLE product_activity_events DROP CONSTRAINT product_activity_events_resource_type_check,
    ADD CONSTRAINT product_activity_events_resource_type_check CHECK (resource_type IN ('workspace', 'web_service', 'static_site', 'private_service', 'background_worker', 'cron_job', 'postgres', 'keyvalue', 'domain', 'build_source')) NOT VALID;
ALTER TABLE product_activity_events VALIDATE CONSTRAINT product_activity_events_resource_type_check;
ALTER TABLE product_inventory_counts DROP CONSTRAINT product_inventory_counts_resource_type_check,
    ADD CONSTRAINT product_inventory_counts_resource_type_check CHECK (resource_type IN ('web_service','static_site','private_service','background_worker','cron_job','postgres','keyvalue','build_source')) NOT VALID;
ALTER TABLE product_inventory_counts VALIDATE CONSTRAINT product_inventory_counts_resource_type_check;
ALTER TABLE product_inventory_lifecycle DROP CONSTRAINT product_inventory_lifecycle_resource_type_check,
    ADD CONSTRAINT product_inventory_lifecycle_resource_type_check CHECK (resource_type IN ('web_service','static_site','private_service','background_worker','cron_job','postgres','keyvalue','build_source')) NOT VALID;
ALTER TABLE product_inventory_lifecycle VALIDATE CONSTRAINT product_inventory_lifecycle_resource_type_check;
