# rt-prune-useless-tests ledger

Earlier runs' **Deferred** and **Skipped** findings, read at the start of every run and updated by its change (see the ground rules in [SKILL.md](SKILL.md)). One line per finding, newest first, at most 40 entries.

## Deferred

None yet.

## Skipped

- 2026-11-15 `dashboard/.../global-search-dialog.test.tsx` — derives expected labels through the component's own `visibleNavGroups(NAV_GROUPS, …)`, but no independent nav-list test exists to make it redundant
