DROP INDEX IF EXISTS webhook_deliveries_endpoint_sent_idx;
ALTER TABLE webhook_deliveries
    DROP COLUMN IF EXISTS response_body,
    DROP COLUMN IF EXISTS sent_at;
