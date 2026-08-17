-- w2/m70: Render's webhook-event history exposes the first send time and
-- bounded response evidence. sent_at is immutable after the first attempt so
-- keyset pagination cannot shift while retries update the row.
ALTER TABLE webhook_deliveries
    ADD COLUMN sent_at timestamptz,
    ADD COLUMN response_body text NOT NULL DEFAULT '';

UPDATE webhook_deliveries
SET sent_at = COALESCE(last_attempted_at, delivered_at, failed_at, created_at)
WHERE attempt_count > 0;

CREATE INDEX webhook_deliveries_endpoint_sent_idx
    ON webhook_deliveries (endpoint_id, sent_at DESC, id DESC)
    WHERE sent_at IS NOT NULL;
