# w5 · m125 — Dashboard decides by error code, not message text, and log views share one empty state

**Worker:** worker5 **Goal:** Blueprint takeover dialogs, deploy-log buckets, log empty states and Scaling utilization each decide from structured data with one shared implementation. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Open Blueprint takeover from error codes, with localized kind and owner | 1h | — |
| t002 | One LogEmptyState for service and datastore logs | 45m | — |
| t003 | Keep deploy-log source types in one map | 30m | — |
| t004 | Share the utilization empty-state reason with the Metrics tab | 45m | — |
| t005 | Render parity | 20m | t001, t002, t003, t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005, t006 |
| t008 | Closeout | 10m | t007 |

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
