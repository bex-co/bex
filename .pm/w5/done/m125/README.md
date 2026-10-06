# w5 · m125 — Dashboard decides by error code, not message text, and log views share one empty state

**Worker:** worker5 **Goal:** Blueprint takeover dialogs, deploy-log buckets, log empty states and Scaling utilization each decide from structured data with one shared implementation. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Open Blueprint takeover from error codes, with localized kind and owner — **DONE** | 1h | — |
| t002 | One LogEmptyState for service and datastore logs — **DONE** | 45m | — |
| t003 | Keep deploy-log source types in one map — **DONE** | 30m | — |
| t004 | Share the utilization empty-state reason with the Metrics tab — **DONE** | 45m | — |
| t005 | Render parity — **DONE** | 20m | t001, t002, t003, t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

- `isTakeoverOnlyConflict` uses `errorDetails{code}`, the dead substring branches are deleted, the dialog state is one value, and zh copy has no raw English kinds or owners.
- Service and datastore log viewers share one `LogEmptyState`, duplicate locale keys are removed, and the key test covers the computed keys.
- `use-deploy-logs.ts` keeps one source-type map, the impossible live pre-deploy branch and its test are gone, and knip reports no unused `LogBucket`.
- Scaling's utilization empty state uses the Metrics tab's reason function, a sleeping service is never labeled "No limit configured", and the scaling test asserts the chart.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of `67fd5ebf8` (w4/189), `d16064828` (w4/180), `4b84b33a9` (w4/195) and `e01f4d27e` (w4/192). Each fix added a special case: substring matching, copied empty states, and parallel type sets.
- **Goal linkage:** ADR018 dashboard parity; i18n quality.
- **Expected outcome:** Server copy changes don't break dashboard logic, and zh users see fully translated dialogs.
- **Why now:** Four dashboard fixes on 2026-10-04 each added a special case; consolidating now prevents a fifth.
- **Render parity included:** dashboard dialogs and empty states change.

## Render parity (t005, 2026-10-06)

- **Blueprint conflict codes:** match on every bex surface. REST (`lego/backend/internal/apps/rest.go:1317`), MCP `validate_bex_yml` (`mcp.go:1023`) and GraphQL `errorDetails` (`graphql.go:1042`) all serialize `BlueprintValidationError`, whose `code` carries `BLUEPRINT_RESOURCE_CONFLICT` or `BLUEPRINT_CONNECTION_CONFLICT`. No API change. Render's validation errors have no takeover codes: the takeover handshake is bex's own, recorded in ADR049.
- **Log empty states:** the service, datastore and deploy log views render one `LogEmptyState`. The datastore viewers' filtered state now uses the service viewer's `logs.emptyFiltered*` copy. Its title was already identical; its body changes from "No database logs match the current filters." and "No log lines match the active filters." to "No logs match these filters." (zh likewise), a deliberate unification with no API change.
- **Deploy-log buckets:** no visible change.
- **Scaling and Metrics utilization:** a parked service with usage but no surviving percentage now reads "Percentages unavailable" instead of "No limit configured". Render has no no-limit state: every Render plan has limits, as every bex tier does (`resourcesForTier`, Guaranteed QoS). No drift to file.

## Evidence (2026-10-06)

- **Codes, not text:** `isTakeoverOnlyConflict` (`dashboard/src/features/blueprints/lib/takeover.ts`) reads `errorDetails{code}`, which both validation queries now select. `protectedConfirmationFromError` keeps only the protected-environment match; its dead Blueprint branches are deleted. Each route holds one pending refusal, and `BlueprintConfirmationDialog` renders it. The zh dialog reads the zh sentence with the localized kind, and a placeholder owner gets its own zh copy.
- **One empty state:** the service, datastore and deploy log views render `LogEmptyState`. The duplicate datastore keys are gone (`*.logsEmptyFiltered*`, and Key Value's `logsErrorTitle`/`logsInstanceLabel`). `translation-keys-exist.test.ts` resolves every quoted `"<namespace>.<key>"` literal, so label tables are checked as well as `t()` calls.
- **Deploy-log buckets:** one bucket-bits map per key. The impossible live pre-deploy branch and its test are gone, and `LogBucket` is no longer exported (knip clean).
- **Utilization:** Scaling and the Metrics tab choose their empty state with `utilizationEmptyReason`. A parked service (`runsInstances`: not suspended, phase not Hibernated) is never "No limit configured". The scaling tests assert the chart (`role="img"`).
- **Live, dev-5 (2026-10-06):** m124-web-paid (running): Scaling draws all three charts, and the Metrics tab shows Memory 2.2% of 512 MiB and CPU against 0.5 cores. m124-web (suspended, Hibernated): Scaling shows "No data captured" and never "No limit configured". The sleeping-with-history state cannot be produced on dev-5, so the component and route tests cover it.
- **Suites:** dashboard `yarn lint` clean; `yarn test` 479 files, 4328 tests. 18 mutants each fail a test (t004, t007). No backend or operator change.
- **Follow-ups:** w5/097 (log, forbidden and protected refusals carry no code); w5/098 (conflict kind sent as display text).
