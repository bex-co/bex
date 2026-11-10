-- Reverse 0227: drop the two kinds' rows, narrow the CHECKs back to 0192's list,
-- then drop the memory columns.
DELETE FROM usage_hourly WHERE kind IN ('sandbox_memory_gb_seconds', 'sandbox_snapshot_gb_seconds');
DELETE FROM usage_monthly WHERE kind IN ('sandbox_memory_gb_seconds', 'sandbox_snapshot_gb_seconds');
ALTER TABLE usage_hourly DROP CONSTRAINT usage_hourly_kind_check;
ALTER TABLE usage_hourly ADD CONSTRAINT usage_hourly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens',
                    'agent_model_cost_microusd'));
ALTER TABLE usage_monthly DROP CONSTRAINT usage_monthly_kind_check;
ALTER TABLE usage_monthly ADD CONSTRAINT usage_monthly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens',
                    'agent_model_cost_microusd'));
ALTER TABLE sandbox_meter_states
    DROP COLUMN memory_remainder,
    DROP COLUMN memory_mib;
