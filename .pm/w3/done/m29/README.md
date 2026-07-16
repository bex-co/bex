# w3 · m29 — Events `type` filter: push the auto-deploy discrimination into SQL

**Worker:** worker3 **Goal:** the three auto-deploy event types (`auto_deploy_enabled` / `auto_deploy_disabled` / `auto_deploy_changed`) actually filter — each `type` query returns only its own rows, on REST, GraphQL, and MCP. **Status:** DONE 2026-07-15

## Tasks (in order)

| id   | title                                                                                                              | est | status |
| ---- | ------------------------------------------------------------------------------------------------------------------ | --- | ------ |
| t001 | Extend `ServiceEventFilter` with an auto-deploy predicate pushed into SQL `WHERE`                                  | 40m | done   |
| t002 | Map the three `type` values to verb+predicate in `pushDown`; keep doc comment truthful                             | 30m | done   |
| t003 | Render parity — same filter semantics across REST/GraphQL/MCP (no adapter changes needed)                          | 30m | done   |
| t004 | Simplify (no further simplification needed)                                                                        | 20m | done   |
| t005 | Test coverage — `TestAutoDeployTypeFilterIsPushedDown` verifies all 3 types                                        | 40m | done   |
| t006 | Closeout                                                                                                           | 15m | done   |

## What shipped

- **`store/events.go`**: `AutoDeployFilter` type (int16, 4 constants: None/Enabled/Disabled/Changed);
  `AutoDeploy AutoDeployFilter` field on `ServiceEventFilter`; `nullAutoDeployFilter` helper;
  `$11::smallint` predicate added to the audit arm of `serviceEventsQuery`:
  ```sql
  AND ($11::smallint IS NULL
       OR ($11 = 1 AND a.auto_deploy_enabled = true)
       OR ($11 = 2 AND a.auto_deploy_enabled = false)
       OR ($11 = 3 AND a.auto_deploy_enabled IS NULL))
  ```
- **`events/service.go`**: `pushDown` split `TypeAutoDeployEnabled`/`TypeAutoDeployDisabled`/`TypeAutoDeployChanged`
  into separate cases, each returning its own `store.AutoDeployFilter`; `List` threads the value
  into `store.ServiceEventFilter.AutoDeploy`.
- **`events/service_test.go`**: `TestAutoDeployTypeFilterIsPushedDown` — 4 sub-tests (one per
  type + the unfiltered case), each asserting the SQL filter reaches `store.ServiceEventFilter`.

No migration needed — `auto_deploy_enabled` was persisted by migration 0037 (w3/m16).

## Definition of done

`GET /v1/services/{id}/events?type=auto_deploy_enabled` (and the GraphQL/MCP equivalents) return only enabled events; likewise `auto_deploy_disabled` and `auto_deploy_changed`; pages stay full-length when mixed auto-deploy rows exist (the filter lives in SQL, not a post-query drop). Tests prove all three.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 18, 2026-07-15 — shipped-diff consistency mine over `7e642e4f..436fd9c2` (w3/m16's same-day commit `436fd9c2`). w3/m16 discriminates `apps.SetAutoDeploy` audit rows into three event types at view time (`events/service.go:490-503`), but `pushDown` maps all three `type` values to the same `verbs=[apps.SetAutoDeploy]` SQL predicate (`service.go:260`) and `List` (`:427-433`) applies no post-filter — so any of the three type filters returns every auto-deploy row.
- **Goal linkage:** Render API parity + the events feed's own filter-contract invariant ("nothing accepted is ignored" — the `pushDown` doc comment). Maps to `GOAL.md` #2 (basic obs) via the observability surface w3 owns.
- **Expected outcome:** filtering by any auto-deploy event type narrows results correctly on REST, GraphQL, and MCP; no accepted filter value silently over-returns.
- **Why now:** the bug shipped today in w3/m16 — fixing it while the m16 context is warm is the loop's proven pattern; every day it stands, clients relying on the documented filter get wrong result sets. The fix must be SQL-side (a Go-side drop reintroduces the short-page problem the pushdown exists to prevent); the `AutoDeployEnabled` boolean is already persisted by migration 0037, so no new migration is needed. Render parity task included: the change touches all three API surfaces.
