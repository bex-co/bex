-- w8/m5 completion audit: resource_kind is part of usage-row identity.
-- Database and KeyValue are distinct Kubernetes kinds and may legally share a
-- name in one namespace; without resource_kind in the key, equal meter/tier
-- rows overwrite each other and compaction merges their totals.

ALTER TABLE usage_hourly DROP CONSTRAINT usage_hourly_pkey;
ALTER TABLE usage_hourly
    ADD PRIMARY KEY (resource_kind, service_id, kind, tier, window_start);

ALTER TABLE usage_monthly DROP CONSTRAINT usage_monthly_pkey;
ALTER TABLE usage_monthly
    ADD PRIMARY KEY (resource_kind, service_id, kind, tier, month);
