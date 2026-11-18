-- w7/m466 (docs/ADR012-auth.md §9c, docs/ADR115-organizations.md D10): an Org
-- Owner can require two-factor authentication and Google login in every
-- workspace of the org, as Render's org Settings "Security" section does.
-- While a flag is on, every org workspace enforces it beside its own setting
-- (the effective requirement is the OR of the two), and so do the org's own
-- objects. Turning it off leaves each workspace's own setting as it was.
ALTER TABLE organizations
  ADD COLUMN IF NOT EXISTS require_two_factor boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS require_google_login boolean NOT NULL DEFAULT false;
