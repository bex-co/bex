# w4 · m204 — Keep service Logs filters aligned with navigation

**Worker:** worker4 **Goal:** a mounted service Logs view adopts the filter and Live state encoded by navigation, so its visible rows and copied URL describe the same reader. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Adopt external log-filter navigation without clobbering local input | 40m | — |
| t002 | Verify route navigation, debounce races and reader continuity | 40m | t001 |
| t003 | Render parity across log surfaces and aliases | 15m | t001, t002 |
| t004 | Simplify the milestone changes | 10m | t003 |
| t005 | Test URL adoption and existing good controls | 20m | t003, t004 |
| t006 | Closeout after live acceptance and cleanup | 15m | t005 |

Total: **6 tasks, 140m**. Filing only; product code is unchanged.

## Definition of done

- Fresh-load an owned Free web service's Logs URL with Application logs, a matching synthetic text and Live off. Click its ordinary sidebar Logs link. The resulting clean URL immediately means All logs, empty search and Live enabled, just as a fresh load of that URL does. Once the destination is committed, history and stream predicates use the destination's state; obsolete source rows must not be presented as settled destination results.
- Repeat at desktop and 390px, with positive and no-match searches. A copied destination URL, reload and native Back/Forward produce the same filters and paused/live semantics. Verify type, text, level, method, statusCode, instance, path and live together; retain range and custom bounds from the destination.
- Own debounced URL writes preserve newer text/path drafts. A late self-issued navigation or timer must not restore obsolete state after an external navigation. Preserve urgent input, the 300ms debounce, omitted defaults, alias normalization and the existing no-write-on-mount guard. Do not force global route remounts or reopen readers on every clock tick.
- Preserve pause/resume rows, filter-change clearing, history/live chronological deduplication, paging, and store-only filters' disabled Live explanation. Datastore Logs and deploy log panels retain their independent state models; static runtime Logs continue redirecting to Events.
- Run relevant dashboard checks and behavior tests. Replay the reported clean-link defect and passing fresh-load control on an owned fixture, delete it, verify own service/public 404 and seven inventory sets, then revoke only that QA session and clear its browser state.

## Source + Goal linkage

- **Source:** continuous functional `$qa-find-bugs w4`, production pass a334, 2026-10-13 UTC. [Finding](finding.md), [complete requests and responses](evidence.json). User requested functional bugs and w4, with muse.env handled only by the login helper.
- **Goal linkage:** ADR008 usable hosting, ADR010 observability and ADR018 honest, shareable log filters.
- **Expected outcome:** clicking Logs cannot leave a narrow paused reader behind a URL that opens an unfiltered live reader for someone else.
- **Why now:** two fresh desktop loads and a fresh mobile load reproduce it. A settled history tick still sends the hidden old text/type after the URL is clean; reload of the same URL broadens the view.
- **Render parity included:** tenant-visible log navigation changes. [Render's logging documentation](https://render.com/docs/logging) describes filtered runtime logs and range/live controls; exact authenticated Render sidebar/navigation semantics were not tested.
- **Dedupe:** 57 open/blocked milestone titles and targeted whole-board open/blocked/done searches found no current owner. This is an uncovered residual of w7/done/m42's shareability work, whose source explicitly leaves mounted URL adoption deferred, not evidence a previously working adoption regressed. Preserve its five DoD clauses as detailed in the finding. w4/228 repaired datastore URL filters through a different component; w4/m203 owns the relative clock, not these predicates.
- **Limits:** live web desktop/mobile only; remaining service families, structured navigation, aliases, native Back/Forward, Chinese and error/pending states are verification work, not observed failures. Cleanup and own-session revocation passed.
