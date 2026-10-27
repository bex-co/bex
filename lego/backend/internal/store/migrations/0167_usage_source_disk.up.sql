-- The provisioned-disk meter (ADR082, w1/m84) records its usage under source
-- 'disk', which 0065's CHECKs never listed: every disk-bearing hour failed its
-- upsert, was logged, and left disk_gb_seconds unmetered. Found from bex-api's
-- "usage: disk meter window" log during w5/m111's live replay.
ALTER TABLE usage_source_streams DROP CONSTRAINT usage_source_streams_source_check;
ALTER TABLE usage_source_streams ADD CONSTRAINT usage_source_streams_source_check
    CHECK (source IN ('instance', 'build', 'storage', 'http', 'websocket', 'direct', 'postgres', 'key_value', 'disk'));
ALTER TABLE usage_source_health DROP CONSTRAINT usage_source_health_source_check;
ALTER TABLE usage_source_health ADD CONSTRAINT usage_source_health_source_check
    CHECK (source IN ('instance', 'build', 'storage', 'http', 'websocket', 'direct', 'postgres', 'key_value', 'disk'));
