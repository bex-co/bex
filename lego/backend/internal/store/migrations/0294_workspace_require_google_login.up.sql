-- w7/m465 (docs/ADR012-auth.md §9b): a workspace admin can require its members
-- to sign in with Google, as Render's Workspace Settings does. While the flag
-- is on, a member whose sign-in is not Google (and any API key or SSH key not
-- created in a Google sign-in) cannot use the workspace's resources.
ALTER TABLE tenants
  ADD COLUMN IF NOT EXISTS require_google_login boolean NOT NULL DEFAULT false;

-- A setWorkspaceRequireGoogleLogin row records the value the admin set the
-- requirement TO, as two_factor_required_to does for 2FA. NULL on every other
-- verb.
ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS google_login_required_to boolean;

-- How the key's creator was signed in when the key was created (core.SignIn:
-- 'oidc:google', 'oidc:github', 'password', 'other'). NULL on every key
-- created before this column existed: those predate the setting and are
-- exempt from it, as Render exempts API keys created before it shipped.
ALTER TABLE ssh_keys
  ADD COLUMN IF NOT EXISTS created_sign_in text;
