-- Reverse w4/m454: drop the granted-scopes audit detail column.
ALTER TABLE audit_events
  DROP CONSTRAINT IF EXISTS audit_events_oauth_grant_scopes_check;
ALTER TABLE audit_events
  DROP COLUMN IF EXISTS oauth_grant_scopes;
