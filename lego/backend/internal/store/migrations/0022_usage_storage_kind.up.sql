-- w8/m9: meter managed-datastore storage separately from compute.
--
-- The usage schema is row-oriented, so adding a meter means extending each
-- table's kind CHECK rather than adding a nullable quantity column. Existing
-- rows need no rewrite and continue to read as zero storage usage by absence.

ALTER TABLE usage_hourly DROP CONSTRAINT usage_hourly_kind_check;
ALTER TABLE usage_hourly ADD CONSTRAINT usage_hourly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds'));

ALTER TABLE usage_monthly DROP CONSTRAINT usage_monthly_kind_check;
ALTER TABLE usage_monthly ADD CONSTRAINT usage_monthly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds'));
