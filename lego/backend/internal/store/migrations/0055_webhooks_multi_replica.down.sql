-- Reverse w1/m58's multi-replica webhook schema. The dedup DELETE in the up
-- migration is not reversible (the duplicate rows were redundant deliveries);
-- dropping the index and column restores the pre-m58 shape.
ALTER TABLE webhook_endpoints DROP COLUMN IF EXISTS notified_at;
DROP INDEX IF EXISTS webhook_deliveries_endpoint_event_uniq;
