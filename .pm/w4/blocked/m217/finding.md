# Distinct repeated log records collapse into one dashboard row

Why: the dashboard hides real application events even though the complete records remain available through the API.

**Severity:** major — visible log-record loss, with no loss of stored messages. Functional QA **a751**,2026-10-16 UTC, workspace `tea-d98210cbbpdc73dcrkvg`. Main synchronized to **ec431de07**. Filing only; no product fix or deployment was made.

## Live reproduction and required behavior

Create the owned Free `node:24-alpine` web fixture from the exact native CreateService request in [capture.json](evidence/capture.json), port8080. Its `/emit` handler emits20 identical `QA_A751_FAST` lines synchronously,20 uniquely numbered `QA_A751_UNIQUE 0…19` lines, then20 identical `QA_A751_SLOW` lines20ms apart. It emits nothing on startup. After the first deploy is Live, request `/emit` once: external HTTP200 / `QA_A751 ready\n`.

Service **srv-db95i31koksc73a7vj4g**, **qa-20261016-a751-records**; sole deploy **dep-db95i31koksc73a7vj50**. Wait for ingestion before comparing counts; the first immediate history response was empty and is not evidence of this defect.

1. Hard-load `/services/OWN_ID/logs?type=app&text=QA_A751_FAST&live=0`. The UI renders **one** fast row.
2. Change Search logs to `qa_a751_fast`. Native Apollo4.3.3 Logs returns **20** original entries, no errors, `hasMore:false`. They share one instance and message but have20 different timestamps from **17:04:33.681127808Z** through **17:04:33.681777363Z**. The UI still has one row.
3. Native searches for SLOW and UNIQUE each return20 completed records; the UI renders all20 in both controls. The unique control shares the burst's timing while changing messages; the slow control repeats the same message in different millisecond buckets.
4. Fresh-load at390px with the fast filter: one row again. A newly triggered native query returns20. Document width390, no captured console errors. The complete accessibility tree and count are durable; the inspected screenshot is a crop of that single rendered row, not independent proof about pagination.
5. A fresh authenticated REST `GET /v1/logs?resource=OWN_ID&type=app&text=QA_A751_FAST&limit=100` returns20 records with **20 distinct existing record IDs**, no more history. Its exact request and complete response are retained.
6. Set SLOW, enable Live, wait, then pause. The replayed control remains20 rows in both states, never40. This verifies the existing same-record replay behavior for this fixture; no transport disconnect was forced.

**Required target:** all20 distinct fast records remain20 visible records at desktop and a fresh mobile load. Re-delivering one record through history, a page, polling or live replay must still show it once. The20 spaced repeats and20 uniquely numbered controls remain20. Preserve original timestamp/message/instance/level, chronological order, range/filter behavior, pause retention and bounded rendering. Do not satisfy the filing by coarsening backend timestamps/IDs, reducing API results to one, hiding repeats intentionally, or disabling deduplication.

## Root cause, consumer and concrete repair

- `dashboard/src/features/logs/lib/map.ts:25–31` builds `logLineKey` from **Date.parse(timestamp)**, instance and message. All20 captured nanosecond stamps parse to the same millisecond **1792170273681**. The captured browser model applies the existing expression to the actual native response and yields **one key** for20 distinct timestamps. Date.parse is behaving as used; the lossy identity choice is the defect.
- `makeLogLine:36–55` assigns that key to both GraphQL and SSE mappings. `fromRenderLog:102–116` ignores the existing payload `id`. `dedupeLogLines:123–130` rejects every later entry sharing the key. `LogViewer:208–218` calls `mergeLogLines`, whose `map.ts:162` dedupe also runs with Live off. This explains the paused-history failure without hypothesizing a disconnect.
- The producer is intact: `lego/backend/internal/logs/render.go:79–101` derives an existing per-record ID from the original timestamp, instance and message hash. REST confirms20 distinct IDs. GraphQL `logGQLType` (`graphql.go:31–42`) preserves the exact timestamp/message/instance but currently exposes no ID. Its fields are ordinary string projections, so adding the existing derived ID is representable; no new resource-id kind or message mutation is needed.
- The renderer cannot restore dropped records: `log-line-list.tsx:129–139` uses `lines.length` as virtualizer count and `line.key` as item key. The20-row passing controls fit its520px viewport plus overscan. The loss occurs before that boundary. The lock pins react-virtual3.14.20 / virtual-core3.19.5; the actual **3.19.5 registry archive** was opened and its count/range code inspected. The installed node_modules wrapper was older3.14.9, so it was not treated as pinned-version evidence. No virtualizer defect or upgrade is proposed.

**Concrete bounded direction:** expose the already-existing `logID(e)` as an additive GraphQL LogEntry string field, select it in the shared Logs document, and carry that identity alongside original data into LogLine. Use the record ID consistently for history, older pages, retained heads, SSE payloads and virtualized rows. Reuse `renderLog.id` from SSE JSON; **EventSource Last-Event-ID / outer SSE id is a timestamp resume cursor**, not this opaque record ID (`rest.go:438–455`), and must remain unchanged.

Apollo compatibility is part of this shape change: the pinned4.3.3 package's `cache/inmemory/helpers.js:defaultDataIdFromObject` automatically normalizes objects with `__typename` and `id`. The shared CSR/SSR configuration (`dashboard/src/common/apollo/cache.ts:6–12`) currently has no LogEntry policy because the old type has no ID. Preserve the existing embedded, query-scoped log caching by explicitly setting `LogEntry: { keyFields: false }` when adding the field, with composed cache tests. The ID still reaches mapping and row keys. This avoids silently changing cache identity/provenance as a side effect of the repair; it is not a diagnosis of the current loss.

Use a documented legacy fallback for old cached/test/ID-less shapes so missing or malformed metadata does not break the reader. Do not collapse two different authoritative IDs merely because their messages and millisecond bucket match. Coordinate backend availability before a dashboard query asks for the new field: GraphQL rejects unknown fields against an older schema, so this rollout boundary belongs in implementation acceptance. Existing REST IDs, REST/MCP messages/timestamps, CLI contracts and raw/display search semantics stay intact.

The old ID-less precision-drift tests can retain a legacy fallback, while production-shaped tests must carry actual IDs and prove both guarantees together: distinct IDs survive even if displayed timestamps have the same precision, and the **same ID** replayed with reformatted/coarser display time remains one row. Removing those controls or merely changing the expected count on the old approximate key is insufficient.

## Shared callers and adjacent cases

Production grep finds **five calls** to `dedupeLogLines`: two in `use-log-history.ts:162,171`, one in `use-older-log-pages.ts:169`, one in `use-deploy-logs.ts:262`, and one in `mergeLogLines` at `map.ts:162`. Live buffering separately tests/adds the same key at `use-live-logs.ts:244–245`. Mapping has two wire entrances, `toLogLine` and `fromRenderLog`.

Trace the full family in its own task: web/private/worker/cron use service LogViewer; service and static deploy details use useDeployLogs; Postgres and Key Value use DatastoreLogViewer → useLogHistory, including older-page retention. These readers may reach the shared key at different phases; do not assert the same first-load symptom for an unprobed sibling. Only web service history and its passing live replay control were exercised here.

Reader changes retain current loading/reset behavior. Genuine same-ID replay is one record; different IDs are different records; ID-less/unparseable/stampless legacy input follows an explicit compatible fallback. Keep timestamp display precision separate from identity, and keep ordering, live ring eviction, pause/resume, source/type bucket membership, instance filtering and page coverage intact. Errors, unavailable sources, unauthenticated/forbidden/missing reads and timeouts retain their current predicates. No denied-read fallback or new API error taxonomy is involved.

## Dedupe and history

Searched9465 board Markdown files across active/blocked/done paths for logLineKey, nanosecond, millisecond, sub-millisecond and repeated-record identity; reviewed the82 actual open/blocked milestone titles, anti-goals, recent40 dashboard/Go commits and targeted key history. No existing open item owns this client collision. Main still contains the millisecond key after sync. Its introduction is **6e35aa9a1**, from [w9/053](../../../w9/done/053.md).

This is an uncovered data-loss consequence of that earlier approximation, rather than a claim that its observed duplicate replay returned:

- w9/053's same-record/coarser-time replay guarantee remains required. The new evidence proves20 distinct records, not20 copies of one record: their raw stamps and existing IDs differ.
- [w6/done/m93](../../../w6/done/m93/README.md) explicitly disproved a later phantom-duplicate diagnosis and added the SSE resume bound instead. That causal distinction remains valid. Preserve its history/live no-phantom-copy behavior and resume cursor, not a renewed duplicate-line premise.
- [w4/done/m192](../../done/m192/README.md) fixes ordering and pause retention. Its scope deliberately left identity unchanged; its full ordering/pause/reader controls remain required. Its existing sub-millisecond sorter does not undo a prior dedupe collision.
- w4/blocked/180 concerns pre-deploy bucket provenance; w4/m200 concerns stdout/stderr record reconstruction; w4/m174 and w8/m65 concern server-side ingestion. The API already returns all20 a751 records, so none owns this cause.
- [m216](../m216/README.md) is Unicode case conversion in deploy-local matching; [327](../../done/327.md) is CR overwrite parsing. These plain ASCII messages contain no ANSI/CR and the raw query results are complete.
- w8/082 concerns the CLI filter editor truncating fractional bounds, not dashboard record identity.

[Render documents a chronological log explorer and individual deploy logs](https://render.com/docs/logging); no authenticated Render same-millisecond replay was performed. Bex's existing stable record IDs and [ADR010](../../../../docs/ADR010-observability.md)'s record/read consistency establish the target. [ECMAScript time values](https://tc39.es/ecma262/multipage/numbers-and-dates.html#sec-time-values-and-time-range) supply the millisecond Date representation; the observed Chromium conversion is recorded directly rather than assumed from display strings.

**Unverified:** live deploy/build/pre-deploy, request-log, private/worker/cron, Postgres/Key Value identity collisions, actual forced-disconnect replay, paged/saturated history, other browsers, native CLI/MCP, authenticated Render and exact deployed dashboard/backend images. Shared impact is source-traced, not claimed as additional live findings.

## Evidence and cleanup

[Complete capture](evidence/capture.json) retains the accepted fixture request, native20-record results, complete REST20-ID result, desktop/mobile row counts and accessibility trees, passing controls, identity model and cleanup. The local inspected image is `.playwright-mcp/qa-a751-fast-mobile.png`; durable evidence does not depend on it. A response-body diagnostic attempted to read an old request after navigation; that harness failure is excluded. The initial empty before ingestion is also excluded.

Native DeleteService returnedtrue at **17:09:19.628Z**. Parent and sole deploy paths return404; all seven original sorted inventory ID sets match at **17:09:57.172Z**. Exact App name and owned pod selector are absent, public URL404 at **17:10:00.542398Z**. Exact-session logout/clear at **17:10:05.734Z** gives401,zero cookies,one blank page and removed local handles. No paid operation, foreign mutation or product-code edit occurred.
