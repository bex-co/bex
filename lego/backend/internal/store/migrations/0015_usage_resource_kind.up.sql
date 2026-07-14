-- w8/m5: extend usage tables with resource_kind so Database/KeyValue
-- instance_seconds rows are distinguishable from App service rows in the
-- REST/GraphQL/MCP/UI usage surface.
--
-- Migration 0021 subsequently includes resource_kind in the primary key: a
-- Database and KeyValue CR may legally share the same Kubernetes name.
-- Backward-compatible: existing rows default to 'service'.

ALTER TABLE usage_hourly  ADD COLUMN IF NOT EXISTS resource_kind text NOT NULL DEFAULT 'service';
ALTER TABLE usage_monthly ADD COLUMN IF NOT EXISTS resource_kind text NOT NULL DEFAULT 'service';
