-- Reverse 0168: drop the token meter rows, narrow the kind CHECKs back to
-- 0096's list, then drop the idempotency ledger.
DELETE FROM usage_hourly WHERE kind LIKE 'agent_model_%';
DELETE FROM usage_monthly WHERE kind LIKE 'agent_model_%';
ALTER TABLE usage_hourly DROP CONSTRAINT usage_hourly_kind_check;
ALTER TABLE usage_hourly ADD CONSTRAINT usage_hourly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds'));
ALTER TABLE usage_monthly DROP CONSTRAINT usage_monthly_kind_check;
ALTER TABLE usage_monthly ADD CONSTRAINT usage_monthly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds'));
DROP TABLE agent_model_usage_events;
