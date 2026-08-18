-- Lossy by construction: the legacy table can retain only its aggregate/last
-- attempt columns. Completion keeps those columns current while 0084 is live,
-- so dropping the child deterministically preserves the latest compatible
-- evidence but necessarily discards older immutable attempts and manual replay
-- idempotency keys.
DROP TRIGGER IF EXISTS webhook_deliveries_attempt_completion_bridge ON webhook_deliveries;
DROP TRIGGER IF EXISTS webhook_deliveries_block_legacy_claim ON webhook_deliveries;
DROP TRIGGER IF EXISTS webhook_deliveries_attempt_insert_bridge ON webhook_deliveries;
DROP TRIGGER IF EXISTS webhook_deliveries_guard_pending_delete ON webhook_deliveries;
DROP FUNCTION IF EXISTS bex_webhook_attempt_from_legacy_completion();
DROP FUNCTION IF EXISTS bex_block_legacy_webhook_claim();
DROP FUNCTION IF EXISTS bex_webhook_attempt_from_legacy_insert();
DROP FUNCTION IF EXISTS bex_guard_webhook_delivery_delete();
DROP TABLE IF EXISTS webhook_delivery_attempts;
DROP FUNCTION IF EXISTS bex_guard_webhook_attempt_update();
ALTER TABLE webhook_deliveries
    DROP CONSTRAINT IF EXISTS webhook_deliveries_id_endpoint_uniq;
DROP FUNCTION IF EXISTS bex_webhook_attempt_id(text);
