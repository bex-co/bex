-- w22/m313 (ADR104 D8, ADR023 § Sandbox snapshot storage): two new sandbox usage
-- kinds.
--
--   sandbox_memory_gb_seconds    allocated GiB of the sandbox shape x seconds
--                                running, recorded from the same phase cursor as
--                                sandbox_compute_seconds. Shadow: recorded and
--                                shown, not billed until the CPU meter lets
--                                billing switch in one step (w22/m314).
--   sandbox_snapshot_gb_seconds  decimal GB x seconds a suspended sandbox's
--                                retained rootfs image is kept (0228 holds the
--                                objects it accrues from).
--
-- The cursor learns the sandbox's allocated memory (MiB, 0 = unknown: a sandbox
-- created before the shape was stamped, which the memory meter skips rather than
-- invents) and carries the memory meter's sub-unit remainder beside the weighted
-- one, in MiB-nanoseconds, so a 30 s poll cadence cannot round usage down.
ALTER TABLE sandbox_meter_states
    ADD COLUMN memory_mib       bigint NOT NULL DEFAULT 0 CHECK (memory_mib >= 0 AND memory_mib <= 1000000),
    ADD COLUMN memory_remainder bigint NOT NULL DEFAULT 0 CHECK (memory_remainder >= 0 AND memory_remainder < 1024000000000);

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
