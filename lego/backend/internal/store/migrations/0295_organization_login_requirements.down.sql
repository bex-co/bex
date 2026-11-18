ALTER TABLE organizations
  DROP COLUMN IF EXISTS require_google_login,
  DROP COLUMN IF EXISTS require_two_factor;
