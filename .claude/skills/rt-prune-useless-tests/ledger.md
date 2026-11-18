# rt-prune-useless-tests ledger

Earlier runs' **Deferred** and **Skipped** findings, read at the start of every run and updated by its change (see the ground rules in [SKILL.md](SKILL.md)). One line per finding, newest first, at most 40 entries.

## Deferred

None yet.

## Skipped

- 2026-11-18 `lego/backend/internal/{keyvalue,postgres}/placement_verbs_test.go` — byte-identical retry/verb bodies each drive their own Service (`fetchKeyValueAs` vs `fetchDatabaseAs`, distinct audit verbs); deleting either loses its package's retry path
- 2026-11-18 `lego/backend/internal/notifications/service_test.go::TestNotifyDeployNoopWithoutMailerOrStore` — assertion-free by design: the nil-mailer/nil-store must-not-panic guard fails on a nil dereference
- 2026-11-18 `dashboard/.../organizations-prefetch.test.tsx` — `toHaveBeenCalled` on the data hook is the layout's own wiring contract (the shell prefetches organizations with the sidebar unmounted); removing the prefetch fails it
- 2026-11-15 `dashboard/.../global-search-dialog.test.tsx` — derives expected labels through the component's own `visibleNavGroups(NAV_GROUPS, …)`, but no independent nav-list test exists to make it redundant
