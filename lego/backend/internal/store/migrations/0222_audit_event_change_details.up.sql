-- w3/m317: the facts Render's disk, maintenance-mode and auto-deploy event
-- members require that no audit column held yet. One typed column per value,
-- like every other per-verb audit detail; NULL for every other verb and for
-- rows recorded before these columns existed (omitted on read, never invented).
--
--   disk_size_from_gb    disk_updated's fromSizeGB (disk_size_gb is the to/size)
--   maintenance_uri_*    maintenance_mode_uri_updated's fromURI/toURI
--   auto_deploy_reason   auto_deploy_disabled's reason: setting_change | rollback
--   auto_deploy_trigger  the trigger before a disable (fromTrigger) or after an
--                        enable (newTrigger): commit | checksPass
ALTER TABLE audit_events
    ADD COLUMN disk_size_from_gb integer,
    ADD COLUMN maintenance_uri_from text,
    ADD COLUMN maintenance_uri_to text,
    ADD COLUMN auto_deploy_reason text,
    ADD COLUMN auto_deploy_trigger text;
