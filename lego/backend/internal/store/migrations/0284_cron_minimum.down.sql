-- Reverse 0284: drop the cron job minimum's rows and month record, then narrow
-- the CHECKs back to 0281's list.
DROP TABLE IF EXISTS cron_minimum_months;
DELETE FROM usage_hourly WHERE kind = 'cron_minimum_microusd';
DELETE FROM usage_monthly WHERE kind = 'cron_minimum_microusd';
ALTER TABLE usage_hourly DROP CONSTRAINT usage_hourly_kind_check;
ALTER TABLE usage_hourly ADD CONSTRAINT usage_hourly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens',
                    'agent_model_cost_microusd', 'sandbox_memory_gb_seconds', 'sandbox_snapshot_gb_seconds',
                    'object_storage_gb_seconds', 'object_storage_ingress_bytes', 'object_storage_egress_bytes',
                    'custom_domain_seconds', 'sandbox_cpu_seconds'));
ALTER TABLE usage_monthly DROP CONSTRAINT usage_monthly_kind_check;
ALTER TABLE usage_monthly ADD CONSTRAINT usage_monthly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens',
                    'agent_model_cost_microusd', 'sandbox_memory_gb_seconds', 'sandbox_snapshot_gb_seconds',
                    'object_storage_gb_seconds', 'object_storage_ingress_bytes', 'object_storage_egress_bytes',
                    'custom_domain_seconds', 'sandbox_cpu_seconds'));
