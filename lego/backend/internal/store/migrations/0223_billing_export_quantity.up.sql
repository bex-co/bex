-- w6/m308 (docs/ADR030 §13): a row's quantity is what was used; the billing
-- provider is sent only what is owed. For build_seconds that is the part above
-- the workspace plan's included pipeline minutes. NULL: the row was sent whole
-- (every row exported before this column, and every other meter).
ALTER TABLE usage_hourly ADD COLUMN billing_export_quantity bigint
    CHECK (billing_export_quantity IS NULL OR billing_export_quantity >= 0);

-- The emitter sums a workspace's earlier build_seconds of the month for each
-- non-zero build row it exports.
CREATE INDEX usage_hourly_build_seconds_idx ON usage_hourly (workspace_id, window_start, resource_kind, service_id)
    INCLUDE (quantity)
    WHERE kind = 'build_seconds' AND quantity > 0;

-- One row per (workspace, month, stage) claims a build-minutes email so
-- concurrent bex-api replicas send it once (w6/m308), as free_hours_notices
-- does for Free instance hours. month is the first day of the UTC month.
CREATE TABLE build_minutes_notices (
    workspace_id text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    month        date        NOT NULL,
    stage        text        NOT NULL CHECK (stage IN ('warned', 'exhausted')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, month, stage)
);
