-- w7/m5: extend usage tables with resource_kind so Database/KeyValue
-- instance_seconds rows are distinguishable from App service rows in the
-- REST/GraphQL/MCP/UI usage surface.
--
-- resource_kind is NOT in the primary key because service_id is already unique
-- per resource type (App ids are typed opaque srv-<xid>; Database/KeyValue ids
-- are the CR name). Backward-compatible: existing rows default to 'service'.

ALTER TABLE usage_hourly  ADD COLUMN resource_kind text NOT NULL DEFAULT 'service';
ALTER TABLE usage_monthly ADD COLUMN resource_kind text NOT NULL DEFAULT 'service';
