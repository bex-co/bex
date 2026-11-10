-- w6/m319 (docs/ADR030 §14): each plan includes Render's outbound bandwidth.
-- The emitter sums a workspace's earlier egress of the month for each
-- non-zero egress row it exports, as it does for build minutes (0223).
CREATE INDEX usage_hourly_egress_bytes_idx ON usage_hourly (workspace_id, window_start, resource_kind, service_id)
    INCLUDE (quantity)
    WHERE kind = 'egress_bytes' AND quantity > 0;

-- One row per (workspace, month, stage) claims an included-bandwidth email so
-- concurrent bex-api replicas send it once. month is the first day of the UTC
-- month.
CREATE TABLE bandwidth_notices (
    workspace_id text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    month        date        NOT NULL,
    stage        text        NOT NULL CHECK (stage IN ('warned', 'exhausted')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, month, stage)
);
