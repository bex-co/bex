-- ADR112 D8 (w21/m416 t011): the three object storage shadow meters.
--
--   object_storage_gb_seconds     decimal GB x seconds the workspace's objects
--                                 sat in Wasabi, accrued by the hourly sweep
--                                 from the bytes it observes between sweeps.
--   object_storage_ingress_bytes  bytes put into object storage.
--   object_storage_egress_bytes   bytes read out of it (exact on the
--                                 Cloudflare-shaped routes, an estimate on
--                                 Render's, which mint a URL).
--
-- Shadow only: recorded and shown at $0, no Stripe Price (like
-- sandbox_memory_gb_seconds, 0227). usage_hourly rows carry
-- resource_kind = 'object_storage' and service_id = the workspace id.
ALTER TABLE usage_hourly DROP CONSTRAINT usage_hourly_kind_check;
ALTER TABLE usage_hourly ADD CONSTRAINT usage_hourly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens',
                    'agent_model_cost_microusd', 'sandbox_memory_gb_seconds', 'sandbox_snapshot_gb_seconds',
                    'object_storage_gb_seconds', 'object_storage_ingress_bytes', 'object_storage_egress_bytes'));

ALTER TABLE usage_monthly DROP CONSTRAINT usage_monthly_kind_check;
ALTER TABLE usage_monthly ADD CONSTRAINT usage_monthly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens',
                    'agent_model_cost_microusd', 'sandbox_memory_gb_seconds', 'sandbox_snapshot_gb_seconds',
                    'object_storage_gb_seconds', 'object_storage_ingress_bytes', 'object_storage_egress_bytes'));

-- The sweep integrates the bytes of each usage row over the time between two
-- observations. gb_remainder carries the sub-unit byte-seconds across sweeps,
-- so an hourly cadence cannot round a small workspace's storage down to zero
-- (the remainder of sandbox_storage_objects does the same).
ALTER TABLE object_storage_usage
    ADD COLUMN gb_remainder bigint NOT NULL DEFAULT 0 CHECK (gb_remainder >= 0 AND gb_remainder < 1000000000);

-- The sweep and the purge look for a workspace's retired bucket rows
-- (deleted_at set), which the live-row indexes of 0273 do not cover.
CREATE INDEX object_buckets_retired_workspace_idx ON object_buckets (workspace_id) WHERE deleted_at IS NOT NULL;
