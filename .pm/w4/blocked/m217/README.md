# w4 · m217 — Preserve distinct log records within one millisecond

**Worker:** worker4 **Goal:** the dashboard retains each actual log record while showing a replay of that record once **Status:** blocked — t001, t002, t003, t005, t006 done 2026-10-16; t004 live replay and t007 need the release pipeline

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Expose existing record identity to GraphQL history — **DONE** | 45m | — |
| t002 | Use authoritative record IDs through dashboard log mapping — **DONE** | 45m | t001 |
| t003 | Audit shared readers, aliases and legacy replay behavior — **DONE** | 50m | t002 |
| t004 | Render parity and live record-count replay | 25m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 35m | t004, t005 |
| t007 | Closeout | 15m | t006 |

## Definition of done

- Recreate the owned Free fixture from [finding.md](finding.md), emit once after Live, and wait for complete ingestion. Native GraphQL and REST each retain20 fast records with20 distinct original stamps/IDs and completed coverage. Service Logs displays all20 at desktop and after a fresh390px load.
- The20 spaced repeats and20 uniquely numbered controls still render20. Live history replay/pause remains20, not40. Re-delivering one authoritative ID with reformatted/coarser display time remains one row; distinct IDs sharing a millisecond/message/instance remain distinct.
- LogEntry remains embedded in the shared CSR/SSR Apollo cache after the new id field is selected. Backend history identity is consumed by the actual mapping, history/head/older retention, live ring and deploy merge; all five shared dedupe call sites plus virtualized keys are verified. REST IDs and messages, GraphQL timestamps, CLI/MCP semantics and SSE timestamp resume cursors retain their contracts.
- Enumerate web/static/cron/worker/private/Postgres/Key Value and both deploy route aliases. Add production-shaped consumer tests and preserve m192 ordering/pause/reset and w9/053/m93 replay controls. Record unprobed live siblings and the older-backend/new-query rollout boundary explicitly.
- Remove owned resources/children, verify public/API404 and original inventory, revoke the exact session, and complete observable live acceptance before closeout.

## Source + Goal linkage

- **Source:** functional QA a751,2026-10-16, user-selected w4; [finding](finding.md) and [complete capture](evidence/capture.json), synchronized main ec431de07. Filing only.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md), [ADR010](../../../../docs/ADR010-observability.md), [ADR006](../../../../docs/ADR006-bex-api.md) and [ADR018](../../../../docs/ADR018-render-parity.md): real events must remain visible to customers and agents diagnosing a deployment.
- **Expected outcome:** display deduplication recognizes a record identity without discarding different records logged inside the same millisecond.
- **Why now:** twenty distinct returned records collapse to one on desktop and a fresh mobile page, while both controls render20. API storage and IDs are proven intact.
- **Sizing:** more than one hour across a shared API/reader identity change, five dedupe consumers and replay compatibility.
- **Parity:** included because GraphQL and dashboard semantics change. Reuse existing REST/SSE record IDs; preserve all current adapter contracts and document Render verification limits.

## Scope

Correct record identity, not timestamp display, ingestion, ANSI parsing, Unicode search, rate limits, retention or transport replacement. No implementation ships during this QA filing. The completed hunt's cleanup does not substitute for later implementation acceptance.

## Implementation record (2026-10-16)

- **t001:** GraphQL `LogEntry.id` resolves to REST's existing `logID` (instance, original timestamp, message hash). It is additive; `TestGraphQLLogEntryIDIsTheRESTRecordID` pins equality with REST and that records a nanosecond apart differ. Shipped first (`bc7d4aa59`, `5a3fc2bb2`; the opaque-resolver budget is acknowledged at 46). The dashboard half shipped only after a deploy containing it completed, so no dashboard queries a backend without the field.
- **t002:** the Logs document selects `id`, and `LogEntry` is `keyFields: false`, so cache embedding is unchanged. `makeLogLine` keys an identified line with a sub-millisecond timestamp as `id:<id>`; other lines keep the millisecond-bucket `logLineKey`. `LogIdentity` treats the first coarse copy and the first identified record in a bucket as one record (either order), so a precision-drifted replay, whose id embeds the coarse stamp, still draws once (w9/053), while further identified records stay distinct.
- **t003:** every reader dedupes through `LogIdentity`. That covers the two `useLogHistory` calls, older pages, `useDeployLogs`, `mergeLogLines`, and the live ring, which forgets evicted lines. Virtualized rows key on `line.key`. Both mapping entrances (GraphQL `toLogLine`, SSE `fromRenderLog` with the payload `id`) carry the id. The SSE `Last-Event-ID` timestamp cursor is untouched. Families: service LogViewer (web/static/cron/worker/private), deploy detail (service and static routes) and DatastoreLogViewer (Postgres/Key Value) share this mapping. Only the shared code was exercised, not those families live.
- **t005/t006:** reviewed inline. `map.test.ts` covers twenty a751-shaped records with distinct ids (dedupe 20, paused merge 20), record plus millisecond replay in both orders (1 row), twenty plus coarse copies (20), the ID-less legacy fallback, and eviction. The existing w9/053 precision-drift and live-tail tests pass. The twenty-record cases fail with legacy keys. Dashboard `yarn test` and `yarn lint` pass; ADR010 is updated.
