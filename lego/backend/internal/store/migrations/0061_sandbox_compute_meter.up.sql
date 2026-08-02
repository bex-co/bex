-- Sandbox compute metering (w7/m76, ADR047 D6).
--
-- usage_hourly remains the one operational meter/outbox table.  The state
-- table is only the durable per-sandbox observation cursor used to turn the
-- OpenSandbox phase signal into non-overlapping running intervals.
ALTER TABLE usage_hourly DROP CONSTRAINT usage_hourly_kind_check;
ALTER TABLE usage_hourly ADD CONSTRAINT usage_hourly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds'));

ALTER TABLE usage_monthly DROP CONSTRAINT usage_monthly_kind_check;
ALTER TABLE usage_monthly ADD CONSTRAINT usage_monthly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds'));

CREATE TABLE sandbox_meter_states (
    workspace_id   text        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    sandbox_id     text        NOT NULL,
    phase          text        NOT NULL,
    tier           text        NOT NULL,
    weight_milli   bigint      NOT NULL CHECK (weight_milli > 0),
    observed_at    timestamptz NOT NULL,
    remainder_nanos bigint     NOT NULL DEFAULT 0 CHECK (remainder_nanos >= 0 AND remainder_nanos < 1000000000),
    PRIMARY KEY (workspace_id, sandbox_id)
);

CREATE INDEX sandbox_meter_states_active_idx
    ON sandbox_meter_states (workspace_id, observed_at)
    WHERE phase <> 'terminated';
