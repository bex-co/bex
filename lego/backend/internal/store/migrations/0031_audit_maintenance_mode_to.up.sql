-- Render's MaintenanceModeEnabledEvent includes metadata.to=true|false. Keep
-- this as one typed nullable boolean rather than adding a free-form audit
-- details object: arbitrary verb arguments (especially secret values) remain
-- structurally unable to enter audit_events.
ALTER TABLE audit_events ADD COLUMN maintenance_mode_to boolean;
