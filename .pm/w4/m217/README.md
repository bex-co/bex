# w4 · m217 — Preserve distinct log records within one millisecond

**Worker:** worker4 **Goal:** the dashboard retains each actual log record while showing a replay of that record once **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Expose existing record identity to GraphQL history | 45m | — |
| t002 | Use authoritative record IDs through dashboard log mapping | 45m | t001 |
| t003 | Audit shared readers, aliases and legacy replay behavior | 50m | t002 |
| t004 | Render parity and live record-count replay | 25m | t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 35m | t004, t005 |
| t007 | Closeout | 15m | t006 |

## Definition of done

- Recreate the owned Free fixture from [finding.md](finding.md), emit once after Live, and wait for complete ingestion. Native GraphQL and REST each retain20 fast records with20 distinct original stamps/IDs and completed coverage. Service Logs displays all20 at desktop and after a fresh390px load.
- The20 spaced repeats and20 uniquely numbered controls still render20. Live history replay/pause remains20, not40. Re-delivering one authoritative ID with reformatted/coarser display time remains one row; distinct IDs sharing a millisecond/message/instance remain distinct.
- LogEntry remains embedded in the shared CSR/SSR Apollo cache after the new id field is selected. Backend history identity is consumed by the actual mapping, history/head/older retention, live ring and deploy merge; all five shared dedupe call sites plus virtualized keys are verified. REST IDs and messages, GraphQL timestamps, CLI/MCP semantics and SSE timestamp resume cursors retain their contracts.
- Enumerate web/static/cron/worker/private/Postgres/Key Value and both deploy route aliases. Add production-shaped consumer tests and preserve m192 ordering/pause/reset and w9/053/m93 replay controls. Record unprobed live siblings and the older-backend/new-query rollout boundary explicitly.
- Remove owned resources/children, verify public/API404 and original inventory, revoke the exact session, and complete observable live acceptance before closeout.

## Source + Goal linkage

- **Source:** functional QA a751,2026-10-16, user-selected w4; [finding](finding.md) and [complete capture](evidence/capture.json), synchronized main ec431de07. Filing only.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md), [ADR010](../../../docs/ADR010-observability.md), [ADR006](../../../docs/ADR006-bex-api.md) and [ADR018](../../../docs/ADR018-render-parity.md): real events must remain visible to customers and agents diagnosing a deployment.
- **Expected outcome:** display deduplication recognizes a record identity without discarding different records logged inside the same millisecond.
- **Why now:** twenty distinct returned records collapse to one on desktop and a fresh mobile page, while both controls render20. API storage and IDs are proven intact.
- **Sizing:** more than one hour across a shared API/reader identity change, five dedupe consumers and replay compatibility.
- **Parity:** included because GraphQL and dashboard semantics change. Reuse existing REST/SSE record IDs; preserve all current adapter contracts and document Render verification limits.

## Scope

Correct record identity, not timestamp display, ingestion, ANSI parsing, Unicode search, rate limits, retention or transport replacement. No implementation ships during this QA filing. The completed hunt's cleanup does not substitute for later implementation acceptance.
