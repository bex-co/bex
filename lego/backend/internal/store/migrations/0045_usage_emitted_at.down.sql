DROP INDEX IF EXISTS usage_hourly_unemitted_idx;
ALTER TABLE usage_hourly DROP COLUMN emitted_at;
