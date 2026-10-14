# w4 · m210 — Keep capped cron history chronological

**Worker:** worker4 **Goal:** retain the newest ten cron executions in stable chronological order without reclassifying old completed Jobs as new **Status:** todo

**Estimate:** 2h55 across seven tasks. [Finding and compatibility audit](finding.md) · [Complete live evidence](../evidence/m210-cron-history-boundary.json).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Model history ordering, tombstones and legacy records | 25m | — |
| [t002](t002.md) | Merge chronological observations before applying the cap | 45m | t001 |
| [t003](t003.md) | Verify run adapters, paging and acknowledgement compatibility | 25m | t002 |
| [t004](t004.md) | Render parity | 15m | t003 |
| [t005](t005.md) | Simplify | 10m | t004 |
| [t006](t006.md) | Test coverage | 45m | t004 |
| [t007](t007.md) | Closeout | 10m | t005, t006 |

## Definition of done

- On an owned Free image cron with an annual future schedule, use native Trigger Run for one successful execution then ten unsuccessful executions, waiting for each terminal result. At the eleventh execution, complete GraphQL and REST histories contain precisely the newest ten run IDs in chronological order: run11 first, old success outside the retained set. Repeated unchanged reads retain that same set/order; no old Job regains priority merely because its status entry was evicted.
- Fresh Events pages at1440×1000 and390×1200 show newest failed runs before older retained ones; the five-row first page and Load more reflect API order. All retained statuses and timestamps remain accurate. A further successful native recovery appears first, displaces only the oldest retained row and advances the bounded success-timestamp derivation correctly.
- Preserve the observed first-failure timestamp control before the cap. lastSuccessfulRunAt continues to derive from the newest **retained** success; when none remains it keeps its documented empty/omitted behavior. Do not add an unbounded success history or fake a date.
- Complete t001/t003/t006's separately marked deleted/pre-start tombstone, legacy clock, tie, restart, cancellation/preemption, mixed scheduled/manual, cursor and adapter controls. No execution replay was observed in this hunt; preserve m108's independent acknowledgement guarantee.
- Required operator/type/codegen/lint and affected backend/dashboard checks pass. Delete the exact owned service, confirm service/runs404 and full baseline ID sets, remove observers/temp files and revoke the own session. Do not close on filing or local-only source changes.

## Source + Goal linkage

- **Source:** continuous functional $qa-find-bugs w4, 2026-10-14 a475; normal dashboard confirmations only. Old success becomes first after run11, reproduced by unchanged reads, fresh desktop/mobile and REST; full transactions and exact cleanup retained.
- **Goal linkage:** ADR008 dependable hosting diagnostics; ADR038 cron execution and ADR006/ADR018 coherent API/dashboard presentation.
- **Expected outcome:** Recent Runs and API pagination identify the actual newest retained executions after history fills.
- **Why now:** the original first-class run-history assumption confuses display eviction with a newly observed Job; the retained set itself is wrong, so a table sort cannot fix it. Existing cancellation/tombstone constraints require a modeled change.
- **Scope:** one operator history producer and optional internal CR status ordering metadata, with adapter/consumer regression verification. Preserve max10, IDs, statuses, failure reasons, execution/acknowledgement, UTC schedule, permissions and poll semantics. No product redesign or long-term history store.
- **Render parity:** included because public reads/UI reflect the producer. Bex GET history is a documented extension; compare primary trigger/cancel contracts and preserve field/wrapper/vocabulary compatibility without inventing upstream retention guarantees. Authenticated Render was not tested.
- **Limits:** only manual runs on one owned Free BusyBox cron were exercised. Scheduled mixes, cancellation, process restart, deleted tombstones, legacy migration, cursor tails, MCP/CLI and broad suites are verification tasks, not this pass's completed guarantees.
