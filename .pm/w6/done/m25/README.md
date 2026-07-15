# w6 · m25 — `maxShutdownDelaySeconds`: graceful-shutdown window per service

**Worker:** worker6 **Goal:** Render's `maxShutdownDelaySeconds` (SIGTERM grace window, 1–300s, default 30) exists end-to-end: CRD field → pod `terminationGracePeriodSeconds`, settable/readable on REST/GraphQL/MCP + dashboard for web, private, and background-worker services. **Status:** done — CRD/operator, REST/GraphQL/MCP, dashboard, parity docs, and regression coverage shipped; browser save/reload persisted 121 seconds and a mock-cluster worker pod was observed Running with `terminationGracePeriodSeconds=137` on 2026-07-14.

## Tasks (in order)

| id | title | est | depends_on | status |
| --- | --- | --- | --- | --- |
| t001 | CRD field + operator → `terminationGracePeriodSeconds` | 45m | — | — **DONE** |
| t002 | REST: create + `PATCH` + read in `serviceDetails` | 40m | t001 | — **DONE** |
| t003 | GraphQL + MCP mirrors | 30m | t002 | — **DONE** |
| t004 | Settings inline-edit row | 30m | t003 | — **DONE** |
| t005 | Render parity | 30m | t004 | — **DONE** |
| t006 | Simplify | 30m | t005 | — **DONE** |
| t007 | Test coverage | 40m | t005 | — **DONE** |
| t008 | Closeout | 15m | t007 | — **DONE** |

## Definition of done

A web/private/background-worker service created or PATCHed with `maxShutdownDelaySeconds: N` (1–300) runs pods whose `terminationGracePeriodSeconds` is N (envtest-asserted); out-of-range or wrong-type values get a named 400; the field reads back in Render's `serviceDetails` placement on all three surfaces and is editable in Settings; cron/static services reject it per the spec's placement (the schema exists only on web/private/worker details).

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 6, 2026-07-14 — field-level spec-grep of Render's live OpenAPI (`components/schemas/maxShutdownDelaySeconds`: integer 1–300 default 30, "maximum time Render waits for your application process to exit gracefully after SIGTERM"; referenced from `webServiceDetails`/`privateServiceDetails`/`backgroundWorkerDetails` + POST/PATCH). Zero hits in `lego/` — never inventoried by ADR018's row-level audit.
- **Goal linkage:** Render parity (service contract field) + workload correctness — a worker draining a long job is killed at k8s's default 30s today with no way to extend.
- **Expected outcome:** a Render client sending the field works unchanged; long-running shutdown paths get their window; one fewer permanent allowlist entry for `w7/m30`'s conformance suite.
- **Why now:** verified-in-spec gap with an exact k8s mechanism (`terminationGracePeriodSeconds`) — the cheapest kind of parity left; continues w6's service-field thread (m21). Render parity task included — all-surface change.
