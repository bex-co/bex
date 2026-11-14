-- w22/m402 (docs/ADR012-auth.md §9a): a workspace admin can require two-factor
-- authentication, as Render's Workspace Settings does. While the flag is on, a
-- member with no enrolled second factor (and any API key or SSH key that member
-- holds) cannot use the workspace's resources.
ALTER TABLE tenants
  ADD COLUMN IF NOT EXISTS require_two_factor boolean NOT NULL DEFAULT false;

-- A setWorkspaceRequireTwoFactor row records the value the admin set the
-- enforcement TO (true = required, false = no longer required), as
-- maintenance_mode_to records its toggle. NULL on every other verb and on
-- rows recorded before this.
ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS two_factor_required_to boolean;
