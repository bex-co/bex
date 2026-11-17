-- w7/m429 t004 (docs/ADR030 §16, docs/ADR023): custom domains beyond the
-- workspace plan's included count (2 Hobby, 15 Pro, 25 Scale, Enterprise
-- unlimited) are metered as Render charges for them.
--
--   custom_domain_seconds  each verified custom domain above the included
--                          count x the seconds it was verified in the hour.
--                          One row per workspace and hour: resource_kind =
--                          'custom_domains', service_id = the workspace id.
--
-- usage_meter_cursors is the start of the last hour window a meter that writes
-- rows only for some hours has metered. The custom domain meter writes no row
-- for a workspace within its included count, so the newest row cannot tell it
-- where it stopped; without a cursor every tick would re-meter the catch-up
-- window with the present state (a plan downgraded now would bill hours
-- before it).
ALTER TABLE usage_hourly DROP CONSTRAINT usage_hourly_kind_check;
ALTER TABLE usage_hourly ADD CONSTRAINT usage_hourly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens',
                    'agent_model_cost_microusd', 'sandbox_memory_gb_seconds', 'sandbox_snapshot_gb_seconds',
                    'object_storage_gb_seconds', 'object_storage_ingress_bytes', 'object_storage_egress_bytes',
                    'custom_domain_seconds'));

ALTER TABLE usage_monthly DROP CONSTRAINT usage_monthly_kind_check;
ALTER TABLE usage_monthly ADD CONSTRAINT usage_monthly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens',
                    'agent_model_cost_microusd', 'sandbox_memory_gb_seconds', 'sandbox_snapshot_gb_seconds',
                    'object_storage_gb_seconds', 'object_storage_ingress_bytes', 'object_storage_egress_bytes',
                    'custom_domain_seconds'));

CREATE TABLE usage_meter_cursors (
    kind        text PRIMARY KEY,
    last_window timestamptz NOT NULL
);
