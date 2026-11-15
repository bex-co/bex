-- ADR112 D4: the month-to-date egress gauge sums every workspace's
-- object_storage_egress_bytes rows since the first of the month, once a minute
-- on every bex-api replica (ObjectStorageEgressSince). No existing index serves
-- a sum over one kind across workspaces by window, and the hot window holds
-- three months of hourly rows.
CREATE INDEX usage_hourly_object_storage_egress_idx ON usage_hourly (window_start)
    INCLUDE (quantity)
    WHERE resource_kind = 'object_storage' AND kind = 'object_storage_egress_bytes';
