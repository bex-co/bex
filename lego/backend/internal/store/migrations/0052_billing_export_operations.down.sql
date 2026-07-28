DROP TABLE IF EXISTS billing_export_issues;

DROP INDEX IF EXISTS usage_hourly_billing_pending_idx;
CREATE INDEX usage_hourly_unemitted_idx
    ON usage_hourly (window_start)
    WHERE emitted_at IS NULL;

ALTER TABLE usage_hourly
    DROP COLUMN IF EXISTS billing_export_error,
    DROP COLUMN IF EXISTS billing_export_error_code,
    DROP COLUMN IF EXISTS billing_export_event_name,
    DROP COLUMN IF EXISTS billing_export_transaction_id,
    DROP COLUMN IF EXISTS billing_export_attempted_at,
    DROP COLUMN IF EXISTS billing_export_state;
