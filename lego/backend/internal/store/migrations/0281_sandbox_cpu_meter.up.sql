-- w22/m314 (ADR104 D8, ADR023 § Sandbox CPU billed on use): the used-CPU meter.
--
--   sandbox_cpu_seconds  CPU-seconds a sandbox's pod used while the sandbox was
--                        running, read from Prometheus (the pod-level cgroup
--                        counter the kubernetes-cadvisor job keeps as
--                        sandbox_pod_cpu_usage_seconds_total).
--
-- cAdvisor series carry only the pod name, and a resume recreates the pod, so
-- usage is attributed by interval: sandbox_running_intervals holds one row per
-- stretch a sandbox ran on one pod. The phase cursor opens a row when it first
-- sees the sandbox running, closes it when it sees anything else (or the pod
-- name change), and consecutive rows share their boundary instant, so the rows
-- of one sandbox neither overlap nor leave a gap.
--
-- The CPU meter walks each row hour by hour: cpu_metered_through is how far
-- its CPU has been added to usage_hourly. A window Prometheus cannot answer for
-- is not advanced (cpu_deferred_since marks the row partial) and is read again
-- on a later poll, so an outage delays a charge and never writes a zero.
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

-- The cursor remembers whether the sandbox is billed on use (a Render sandbox:
-- used CPU plus allocated memory) or keeps the weighted allocated-shape meter
-- (an agent-session sandbox, whose cost is its session's, ADR047 D6). A cursor
-- written before this column reads false until its next observation.
ALTER TABLE sandbox_meter_states
    ADD COLUMN usage_billed boolean NOT NULL DEFAULT false;

CREATE TABLE sandbox_running_intervals (
    workspace_id text        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    sandbox_id   text        NOT NULL,
    -- The pod that ran the sandbox over this interval, in the workspace's
    -- sandbox namespace.
    pod_name     text        NOT NULL CHECK (pod_name <> ''),
    tier         text        NOT NULL DEFAULT '',
    -- Whether the CPU meter reads this interval (a sandbox billed on use).
    usage_billed boolean     NOT NULL,
    started_at   timestamptz NOT NULL,
    ended_at     timestamptz CHECK (ended_at IS NULL OR ended_at >= started_at),
    cpu_metered_through  timestamptz NOT NULL,
    -- Sub-unit carry between windows, in millionths of a CPU-second.
    cpu_remainder_micros bigint      NOT NULL DEFAULT 0 CHECK (cpu_remainder_micros >= 0 AND cpu_remainder_micros < 1000000),
    -- Since when the next window has been waiting for Prometheus (the interval
    -- is partial); NULL once it has been read.
    cpu_deferred_since   timestamptz,
    -- When the meter last tried this interval: the queue rotates on it, so an
    -- interval Prometheus cannot answer for does not starve the others.
    cpu_attempted_at     timestamptz,
    -- No window left: not billed on use, or ended and metered through its end.
    cpu_settled  boolean     NOT NULL DEFAULT false,
    PRIMARY KEY (workspace_id, sandbox_id, started_at)
);

-- One open interval per sandbox.
CREATE UNIQUE INDEX sandbox_running_intervals_open_idx
    ON sandbox_running_intervals (workspace_id, sandbox_id)
    WHERE ended_at IS NULL;
-- The CPU meter's queue, in rotation order.
CREATE INDEX sandbox_running_intervals_cpu_idx
    ON sandbox_running_intervals (cpu_attempted_at NULLS FIRST, cpu_metered_through)
    WHERE NOT cpu_settled;
-- Pruning: closed intervals, oldest first.
CREATE INDEX sandbox_running_intervals_prune_idx
    ON sandbox_running_intervals (ended_at)
    WHERE ended_at IS NOT NULL;
