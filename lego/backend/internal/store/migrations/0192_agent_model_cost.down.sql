-- Reverse 0191: drop the cost meter rows, narrow the kind CHECKs back to
-- 0168's list, then drop the cost columns.
DELETE FROM usage_hourly WHERE kind = 'agent_model_cost_microusd';
DELETE FROM usage_monthly WHERE kind = 'agent_model_cost_microusd';
ALTER TABLE usage_hourly DROP CONSTRAINT usage_hourly_kind_check;
ALTER TABLE usage_hourly ADD CONSTRAINT usage_hourly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens'));
ALTER TABLE usage_monthly DROP CONSTRAINT usage_monthly_kind_check;
ALTER TABLE usage_monthly ADD CONSTRAINT usage_monthly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens'));
ALTER TABLE agent_model_usage_events
    DROP COLUMN price_microusd,
    DROP COLUMN cost_source,
    DROP COLUMN upstream_cost_microusd;
