-- w3/m82: typed detail columns for the field-level managed-datastore
-- configuration effects. A PATCH that flipped high availability, toggled the
-- connection pooler, grew the disk, or changed a Key Value eviction/persistence
-- setting used to land as one undifferentiated postgres.UpdatePostgres /
-- keyvalue.UpdateKeyValue row, so Render's postgres_ha_status_changed,
-- postgres_connection_pool_enabled_changed, postgres_disk_size_changed, and
-- key_value_config_restart had no producer. The service layer now records a
-- fixed verb per changed field carrying the value it set — the same one
-- column per value discipline as maintenance_mode_to and auto_deploy_enabled,
-- never a free-form object. NULL means the verb does not define the field.
ALTER TABLE audit_events ADD COLUMN high_availability_enabled boolean;
ALTER TABLE audit_events ADD COLUMN connection_pool_enabled boolean;
ALTER TABLE audit_events ADD COLUMN disk_size_gb integer;
ALTER TABLE audit_events ADD COLUMN maxmemory_policy text;
ALTER TABLE audit_events ADD COLUMN persistence_mode text;
