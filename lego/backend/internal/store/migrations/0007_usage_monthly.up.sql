-- w8/m4: usage retention — monthly aggregates for compacted hourly detail.
--
-- usage_hourly grows without bound (one row per service × kind × tier ×
-- hour). Months older than the hot window (BEX_USAGE_RETENTION_MONTHS,
-- default 3, docs/ADR023-usage-metering.md) are compacted into this table — a
-- lossless SUM per calendar month — and the hourly rows purged, so the
-- control-plane store's size stays predictable as tenants and time grow.
--
-- Mirrors usage_hourly's conventions (0006_usage.up.sql): same kind CHECK,
-- same empty-string tier for non-compute meters, same tenant FK. month is the
-- first day of the calendar month (UTC). The primary key lets the compaction
-- upsert additively (ON CONFLICT … DO UPDATE quantity + EXCLUDED.quantity):
-- a straggler hourly row compacted later adds to the month it belongs to.

CREATE TABLE usage_monthly (
    workspace_id text   NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    service_id   text   NOT NULL,
    kind         text   NOT NULL CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds')),
    tier         text   NOT NULL DEFAULT '',
    month        date   NOT NULL,
    quantity     bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (service_id, kind, tier, month)
);

-- The period query is workspace-scoped by month, same shape as
-- usage_hourly's (workspace_id, window_start) index.
CREATE INDEX usage_monthly_workspace_month_idx ON usage_monthly (workspace_id, month);
