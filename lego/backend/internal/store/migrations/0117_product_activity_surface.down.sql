ALTER TABLE product_analytics_collection
    DROP COLUMN IF EXISTS surface_started_at;

ALTER TABLE product_activity_events
    DROP CONSTRAINT IF EXISTS product_activity_events_surface_check;

ALTER TABLE product_activity_events
    DROP COLUMN IF EXISTS surface;
