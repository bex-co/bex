# w8 · m65 — Retain PostgreSQL process logs during bursts

**Worker:** worker8 **Goal:** successful database statements retain their distinct diagnostic records before hosting history search. **Status:** blocked — t001 done (2026-10-13: installed chart 0.29.0 = operator 1.30.0; upstream fix #11131 is in operator 1.30.1 = chart 0.29.1); t002–t007 need the user to approve a CNPG upgrade rollout that restarts every managed Postgres (or to first enable in-place instance-manager updates), then the credentialed chart mirror, live burst acceptance and cleanup.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Resolve the CNPG manager identity and compatible patched release — **DONE 2026-10-13** | 25m | — |
| t002 | Pin the patched manager and define a controlled rollout — **BLOCKED: rollout decision** | 35m | t001 |
| t003 | Verify the process-log source and affected datastore siblings | 25m | t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify | 10m | t004 |
| t006 | Test coverage | 25m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

Fresh owned Free PostgreSQL statements through published/current Bex and the unmodified2.27 pin emit300 and1005 distinct numbered LOG records in short bursts. All numbers are retained and retrievable through honest supported cursor pages, with exact original bytes/IDs and no silent sampling. Both native clients agree with the complete source. Pacing is a comparison, not the fix. Confirm the installed operator/instance managers contain the selected upstream correction; retain normal-rate Unicode, SQL-error propagation, primary/replica/pooler/backup compatibility and relevant REST/GraphQL/MCP/dashboard checks. Old-manager runtime coverage fails before the correction. Required tests/validation pass and every owned fixture is deleted with detail/list/endpoint/baseline/quota reconciliation before closeout.

## Source + Goal linkage

- **Source:** user-requested continuous CLI QA in w8, sweep218; [two-client source-loss finding](finding.md) and [durable evidence](evidence/cli-sweep218/README.md).
- **Goal linkage:** [ADR008 core hosting](../../../../docs/ADR008-vision.md), [ADR009 managed Postgres](../../../../docs/ADR009-postgresql-management.md) and [ADR010 observability](../../../../docs/ADR010-observability.md).
- **Expected outcome:** customers can retrieve distinct database diagnostic records from real bursts through the unchanged hosting interfaces.
- **Why now:** two successful native statements retain109/1005 and102/300, while the paced1005 control is complete. A released upstream correction exists, but the actual pinned/deployed manager identity still needs resolution.
- **Parity included:** the source feeds tenant-facing REST/GraphQL/MCP/dashboard logs even though the correction belongs to infrastructure.
- **Sizing:**150m /2h30 across seven tasks, including compatibility inventory, artifact/rollout work, source/sibling verification and standing closing tasks.
