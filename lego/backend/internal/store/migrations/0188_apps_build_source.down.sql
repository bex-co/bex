DELETE FROM product_activity_events WHERE resource_type = 'build_source';
DELETE FROM product_inventory_counts WHERE resource_type = 'build_source';
DELETE FROM product_inventory_lifecycle WHERE resource_type = 'build_source';
ALTER TABLE product_activity_events DROP CONSTRAINT product_activity_events_resource_type_check,
    ADD CONSTRAINT product_activity_events_resource_type_check CHECK (resource_type IN ('workspace', 'web_service', 'static_site', 'private_service', 'background_worker', 'cron_job', 'postgres', 'keyvalue', 'domain'));
ALTER TABLE product_inventory_counts DROP CONSTRAINT product_inventory_counts_resource_type_check,
    ADD CONSTRAINT product_inventory_counts_resource_type_check CHECK (resource_type IN ('web_service','static_site','private_service','background_worker','cron_job','postgres','keyvalue'));
ALTER TABLE product_inventory_lifecycle DROP CONSTRAINT product_inventory_lifecycle_resource_type_check,
    ADD CONSTRAINT product_inventory_lifecycle_resource_type_check CHECK (resource_type IN ('web_service','static_site','private_service','background_worker','cron_job','postgres','keyvalue'));
-- NOT VALID keeps rollback from failing on (or deleting) linked services
-- that have no repo or image of their own.
ALTER TABLE apps DROP CONSTRAINT apps_check, ADD CONSTRAINT apps_check CHECK (repo IS NOT NULL OR image IS NOT NULL) NOT VALID;
DROP INDEX IF EXISTS apps_build_source_idx;
ALTER TABLE apps DROP CONSTRAINT IF EXISTS apps_build_source_shape_check;
ALTER TABLE apps DROP COLUMN IF EXISTS build_source_image, DROP COLUMN IF EXISTS build_source_id;
