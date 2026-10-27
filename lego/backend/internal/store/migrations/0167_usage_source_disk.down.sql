-- Reverse 0167: drop disk source rows (health first, through the FK cascade),
-- then narrow both CHECKs back to 0065's list.
DELETE FROM usage_source_streams WHERE source = 'disk';
ALTER TABLE usage_source_health DROP CONSTRAINT usage_source_health_source_check;
ALTER TABLE usage_source_health ADD CONSTRAINT usage_source_health_source_check
    CHECK (source IN ('instance', 'build', 'storage', 'http', 'websocket', 'direct', 'postgres', 'key_value'));
ALTER TABLE usage_source_streams DROP CONSTRAINT usage_source_streams_source_check;
ALTER TABLE usage_source_streams ADD CONSTRAINT usage_source_streams_source_check
    CHECK (source IN ('instance', 'build', 'storage', 'http', 'websocket', 'direct', 'postgres', 'key_value'));
