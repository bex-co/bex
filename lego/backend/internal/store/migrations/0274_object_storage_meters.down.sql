-- Reverse 0274: drop the three kinds' rows, narrow the CHECKs back to 0227's
-- list, then drop the remainder column.
DELETE FROM usage_hourly WHERE kind IN ('object_storage_gb_seconds', 'object_storage_ingress_bytes', 'object_storage_egress_bytes');
DELETE FROM usage_monthly WHERE kind IN ('object_storage_gb_seconds', 'object_storage_ingress_bytes', 'object_storage_egress_bytes');
ALTER TABLE usage_hourly DROP CONSTRAINT usage_hourly_kind_check;
ALTER TABLE usage_hourly ADD CONSTRAINT usage_hourly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens',
                    'agent_model_cost_microusd', 'sandbox_memory_gb_seconds', 'sandbox_snapshot_gb_seconds'));
ALTER TABLE usage_monthly DROP CONSTRAINT usage_monthly_kind_check;
ALTER TABLE usage_monthly ADD CONSTRAINT usage_monthly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens',
                    'agent_model_cost_microusd', 'sandbox_memory_gb_seconds', 'sandbox_snapshot_gb_seconds'));
ALTER TABLE object_storage_usage DROP COLUMN gb_remainder;
DROP INDEX IF EXISTS object_buckets_retired_workspace_idx;
