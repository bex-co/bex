-- Reverse w22/m402: drop the enforcement flag and the audit detail column.
-- Members' enrolled second factors live in Kratos and are unaffected.
ALTER TABLE audit_events
  DROP COLUMN IF EXISTS two_factor_required_to;
ALTER TABLE tenants
  DROP COLUMN IF EXISTS require_two_factor;
