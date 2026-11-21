-- w32/075 (docs/ADR049-render-yaml-parity.md § Blueprint onboarding): Render
-- deletes "a planned Sync that is not approved within 24 hours" (the pinned
-- CLI's CreateBlueprintSync). bex-api's Blueprint recovery sweep deletes, every
-- minute and oldest first, each blueprint_syncs row still `created` (waiting
-- for approval) whose started_at, when it was planned, is 24 hours old or more.
-- Every other row is a run or a settled attempt the sweep never reads, so the
-- index holds only the waiting plans.
CREATE INDEX IF NOT EXISTS blueprint_syncs_waiting_plan_idx
    ON blueprint_syncs (started_at)
    WHERE state = 'created';
