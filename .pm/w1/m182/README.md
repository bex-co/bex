# w1 · m182 — Older Blueprint auto-sync work cannot overwrite newer configuration

**Worker:** worker1 **Goal:** automatic Blueprint synchronization converges to the current eligible branch source despite reordered deliveries, workers and retries. **Status:** todo

**Estimate:** 175m implementation; ~4h25m including standing closing tasks.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Establish authoritative source freshness for automatic intents | 60m | — |
| t002 | Serialize automatic claims per Blueprint and fence reclaimed work | 45m | t001 |
| t003 | Prevent stale automatic work from applying after newer sources | 45m | t001, t002 |
| t004 | Preserve source attribution and explain superseded automatic work | 25m | t003 |
| t005 | Render parity | 25m | t004 |
| t006 | Simplify | 20m | t005 |
| t007 | Test coverage | 40m | t005, t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

- Reversed processing of automatic intents for sources A then B cannot leave obsolete A applied after eligible B; two API workers and expired/reclaimed work preserve this result.
- A delayed older delivery cannot roll configuration backward; a genuine force push follows the authoritative current branch rather than timestamps or SHA ordering.
- Accepted/applied provenance remains truthful. Explicit manual reviewed-commit sync remains pinned; Auto Sync off and disconnect remain effective.
- Real-Postgres regressions and an isolated worker walkthrough establish the result, including restart, policy controls and source history. Required backend/lint gates pass.

## Source + Goal linkage

- **Source:** user-approved brainstorm item 2, 2026-10-20, narrowed after explicit Render parity verification. `store/blueprint_auto_sync.go` uses an unordered UPDATE RETURNING batch; `apps/blueprint_auto_sync_worker.go` applies each accepted SHA without a source freshness fence. This is source-derived evidence, not a claimed production reproduction.
- **Goal linkage:** ADR008 deterministic deployment and Git push to deploy; ADR049's immutable source and honest history contracts.
- **Expected outcome:** retries and delivery order cannot silently revert a Blueprint to obsolete configuration.
- **Why now:** durable intake already exists, but the source-ordering guarantee promised in shipped `w8/done/m38` is not established by its current worker; this is a scoped correction, not another auto-sync feature.
- **Render parity included:** Render's [Blueprint documentation](https://render.com/docs/infrastructure-as-code) confirms automatic application of changed linked Blueprints. Ordering, crash recovery, fencing and force-push behavior here are **bex correctness requirements; Render's corresponding guarantees are undocumented and were not measured**. Preserve the ADR018 Blueprint row's existing partial markers.
- **Boundaries:** reuse the existing worker/store/apply fence. No external queue service, new deployment product or automatic resource deletion.
