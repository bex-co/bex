-- w9/m448 (docs/ADR030 § Cron job minimum): a cron job active in a calendar
-- month whose runs cost less than the cron job minimum (Render's $1 per cron
-- job service × 0.70) is charged the difference.
--
--   cron_minimum_microusd  the top-up, in micro-USD. One row per cron job and
--                          month: resource_kind = 'service', service_id = the
--                          cron job, window_start = the month's last hour.
--
-- cron_minimum_months records each cron job seen active (not suspended) in a
-- month. A deleted cron job's apps row is gone by the time its month closes,
-- so the hourly sweep keeps the record the month's top-up is computed from.
ALTER TABLE usage_hourly DROP CONSTRAINT usage_hourly_kind_check;
ALTER TABLE usage_hourly ADD CONSTRAINT usage_hourly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens',
                    'agent_model_cost_microusd', 'sandbox_memory_gb_seconds', 'sandbox_snapshot_gb_seconds',
                    'object_storage_gb_seconds', 'object_storage_ingress_bytes', 'object_storage_egress_bytes',
                    'custom_domain_seconds', 'sandbox_cpu_seconds',
                    'cron_minimum_microusd'));

ALTER TABLE usage_monthly DROP CONSTRAINT usage_monthly_kind_check;
ALTER TABLE usage_monthly ADD CONSTRAINT usage_monthly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens',
                    'agent_model_cost_microusd', 'sandbox_memory_gb_seconds', 'sandbox_snapshot_gb_seconds',
                    'object_storage_gb_seconds', 'object_storage_ingress_bytes', 'object_storage_egress_bytes',
                    'custom_domain_seconds', 'sandbox_cpu_seconds',
                    'cron_minimum_microusd'));

CREATE TABLE cron_minimum_months (
    service_id   text NOT NULL,
    workspace_id text NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    month        date NOT NULL,
    PRIMARY KEY (service_id, month)
);

CREATE INDEX cron_minimum_months_month_idx ON cron_minimum_months (month);
