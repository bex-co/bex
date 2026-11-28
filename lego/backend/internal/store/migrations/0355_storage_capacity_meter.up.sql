-- w4/m713 (docs/ADR023 § Postgres storage capacity): a Postgres database is
-- billed for the storage capacity it has, as on Render ("Storage capacity for
-- Render Postgres databases", render.com/docs/faq), not for the bytes it uses.
--
--   storage_capacity_gb_seconds  the provisioned capacity of every instance's
--                                volume (primary, HA standby, each read
--                                replica) in the product's GB × the seconds it
--                                was held. resource_kind = 'postgres', tier =
--                                the plan.
--
-- storage_gb_seconds keeps measuring used bytes for Postgres and Key Value.
-- From pricing.yaml storageCapacity.billedFrom it is a reading shown at $0,
-- and the capacity kind is what bills; the reads classify both by hour.
ALTER TABLE usage_hourly DROP CONSTRAINT usage_hourly_kind_check;
ALTER TABLE usage_hourly ADD CONSTRAINT usage_hourly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens',
                    'agent_model_cost_microusd', 'sandbox_memory_gb_seconds', 'sandbox_snapshot_gb_seconds',
                    'object_storage_gb_seconds', 'object_storage_ingress_bytes', 'object_storage_egress_bytes',
                    'custom_domain_seconds', 'sandbox_cpu_seconds',
                    'cron_minimum_microusd', 'storage_capacity_gb_seconds'));

ALTER TABLE usage_monthly DROP CONSTRAINT usage_monthly_kind_check;
ALTER TABLE usage_monthly ADD CONSTRAINT usage_monthly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens',
                    'agent_model_cost_microusd', 'sandbox_memory_gb_seconds', 'sandbox_snapshot_gb_seconds',
                    'object_storage_gb_seconds', 'object_storage_ingress_bytes', 'object_storage_egress_bytes',
                    'custom_domain_seconds', 'sandbox_cpu_seconds',
                    'cron_minimum_microusd', 'storage_capacity_gb_seconds'));
