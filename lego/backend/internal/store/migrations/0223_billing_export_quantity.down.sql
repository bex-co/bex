DROP TABLE IF EXISTS build_minutes_notices;
DROP INDEX IF EXISTS usage_hourly_build_seconds_idx;
ALTER TABLE usage_hourly DROP COLUMN IF EXISTS billing_export_quantity;
