ALTER TABLE ssh_keys
  DROP COLUMN IF EXISTS created_sign_in;
ALTER TABLE audit_events
  DROP COLUMN IF EXISTS google_login_required_to;
ALTER TABLE tenants
  DROP COLUMN IF EXISTS require_google_login;
