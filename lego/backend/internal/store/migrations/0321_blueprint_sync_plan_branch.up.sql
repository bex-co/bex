-- w32/074 (docs/ADR049-render-yaml-parity.md § Blueprint onboarding): the
-- branch a planning attempt was planned from, beside its plan_path. Approving a
-- plan applies it from its own source, the commit, path and branch it was
-- planned at, even after `ea blueprints update` changed the Blueprint's path or
-- branch, as the pinned Render CLI says of Render's approve ("Updating does not
-- change a sync that is waiting for approval, so approve could still apply a
-- sync planned from the previous branch or path"). The Blueprint keeps the
-- settings the update saved. NULL: a planning attempt recorded before this
-- migration, which approval applies with the Blueprint's branch.
ALTER TABLE blueprint_syncs
  ADD COLUMN IF NOT EXISTS plan_branch text;
