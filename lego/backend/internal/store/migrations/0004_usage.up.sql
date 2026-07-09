-- w8/m1: usage metering — durable per-service hourly rollup rows.
--
-- Three metering kinds match Render's three meters exactly:
--   instance_seconds — per-second compute time * instance count (per tier)
--   egress_bytes     — outbound bandwidth (from Traefik)
--   build_seconds    — build-Job duration (Render's "pipeline minutes")
--
-- window_start is the start of the closed hourly window (truncated to the
-- hour). Each (service_id, kind, tier, window_start) is unique so the
-- rollup loop can re-evaluate any window idempotently via ON CONFLICT … DO
-- UPDATE without double-counting.
--
-- tier is meaningful only for instance_seconds (the compute ladder row, e.g.
-- "free", "starter"); for the other kinds it is the empty string, which still
-- participates in the unique constraint to keep one column type for the key.
--
-- quantity is in the natural unit of each kind (seconds, bytes, seconds) as
-- an integer; fractional seconds/bytes from Prometheus are truncated.

CREATE TABLE usage_hourly (
    workspace_id text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    service_id   text        NOT NULL,
    kind         text        NOT NULL CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds')),
    tier         text        NOT NULL DEFAULT '',
    window_start timestamptz NOT NULL,
    quantity     bigint      NOT NULL DEFAULT 0,
    PRIMARY KEY (service_id, kind, tier, window_start)
);

-- Month-to-date aggregation and the workspace-scoped cap check both start
-- with workspace_id and window_start, so the compound index covers both.
CREATE INDEX usage_hourly_workspace_window_idx ON usage_hourly (workspace_id, window_start);
