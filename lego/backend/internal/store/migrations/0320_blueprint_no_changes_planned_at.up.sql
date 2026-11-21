-- w32/071 (docs/ADR049-render-yaml-parity.md § Blueprint onboarding): when the
-- newest plan of a Blueprint found no changes. Such a plan writes no sync row,
-- as Render's createBlueprintSync answers `sync: null` for `no_changes`, so
-- without this stamp an approval read the planning attempt before it (a failed
-- plan) as the newest and told the user to fix the file and re-plan, which they
-- just had. SettleNoChangesPlan stamps it on every no-changes plan, and
-- ApproveBlueprint answers BLUEPRINT_NOTHING_TO_APPROVE while it is newer than
-- the newest recorded planning attempt. NULL: no plan has found no changes
-- since this migration.
ALTER TABLE blueprints
  ADD COLUMN IF NOT EXISTS no_changes_planned_at timestamptz;
