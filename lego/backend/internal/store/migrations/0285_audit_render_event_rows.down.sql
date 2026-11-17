-- Reverse w9/m449.
ALTER TABLE audit_events
  DROP COLUMN IF EXISTS feed_only,
  DROP COLUMN IF EXISTS env_var_keys;
