# Service Logs keeps old filters after its navigation link clears the URL

Why: an operator sharing or reloading the displayed URL gets a different log reader from the one the page currently shows.

**Severity:** major (misleading log reader/shareable URL). **Estimate:** 140m across six tasks. Functional QA only; filing only. Research HEAD `043a21562`.

## Reproduction and controls

Owned Free image web service `qa-20261013-a334-filters`, `srv-db6urlovrh0s73bstfo0`, workspace bex. Python HTTP fixture logs distinct alpha, beta and literal markers on requests. Corrected deploy `dep-db6us59g8k9s73eehpk0` reached Live at **08:38:57.323385Z**, plan readback Free, revision 2. Initial multiline test input was flattened by the HTML single-line field, then repaired through ordinary Settings; its SyntaxError records are bad-input controls and are not a finding.

1. Fresh-load `/services/<owned-id>/logs?type=app&live=0&text=qa-a334-alpha`. Its stored matching marker is visible with Application logs and Live off.
2. Change text to `qa-a334-literal-a.*-b`. At **08:40:37.596Z**, the actual GraphQL response contains exactly its one intended literal marker, hasMore false; the URL and controls agree. Literal semantics are bex's existing API contract; this is not an authenticated Render wildcard comparison.
3. Click the existing sidebar **Logs** link, whose actual href is the clean same-service Logs URL. At **08:41:13.125Z**, the URL has no query parameters, but text remains literal, type remains Application and Live remains off; the narrow marker still renders.
4. Independent fresh load with beta at **08:41:41.962Z** returns the one beta marker. Click Logs again. At **08:42:11.762Z**, the URL is clean while its captured settled history tick still sends `type:app, text:qa-a334-beta`, returning one beta marker. This is not only an old DOM row waiting for a fetch.
5. Reload the same clean URL at **08:42:32.724Z**: input becomes empty and Live becomes true. The actual request omits type/text and returns **17** stored rows, including all three synthetic markers and request rows. The viewer's same clean URL has materially different semantics depending on whether its component was retained.
6. Fresh 390px page uses alpha. Open Sidebar, click Logs, dismiss Sidebar manually, then inspect settled state at **08:44:38.684Z**: URL clean, alpha text/Application/off remain, exactly the alpha row visible. Document width stays 390. Viewed supplementary screenshot `.playwright-mcp/qa-log-url-nav-a334-mobile-settled.png` (57,805 bytes) supports the controls/row, while durable JSON captures the URL.

**Target:** the committed navigation destination is authoritative. A clean destination immediately means empty filters/All logs/Live true, consistent with the existing fresh-load control. A filtered destination adopts its exact validated filter set and live value. Keep local drafts only while their own debounced URL write catches up; an external destination supersedes those drafts. Do not repair this by keeping hidden old filters, silently restoring them into the clean destination, or changing fresh-load defaults.

## Producer, consumer and library mechanism

- `dashboard/src/common/components/dashboard-layout/sidebar-nav-groups.tsx:60` renders a TanStack Link using the nav target without search. `features/services/components/service-nav.tsx:64` defines the same-service Logs target. The live href/click receipts confirm that this action clears the search.
- `dashboard/src/routes/services.$serviceId.logs.tsx:33–35,65–87` reads the new validated search and passes fresh `initialFilters` and `initialLive`; range already reads the new search. Its writer uses a complete cleared-key spread plus replace and semantic equality guard. The serializer's default semantics are `features/logs/lib/log-search.ts:145–150,158–170` and `features/logs/types.ts:107`: absent text/structured filters = empty, absent type = all, absent live=0 = live enabled.
- `features/logs/components/log-viewer.tsx:45–47,75–78` explicitly reads those props only at mount. `84–105` derives queryFilters from retained local state; `118–125` only mirrors local changes outward and never adopts a changed URL. `140–155` sends that state to both history and the live predicate, so changing only a visible control or history request would leave another consumer wrong.
- Actual installed **React DOM 19.2.3** `node_modules/react-dom/cjs/react-dom-client.development.js:8264–8289` stores the initializer on mount; its update dispatcher `26505–26512` calls updateReducer without reapplying initializer props. Actual **@tanstack/react-router 1.170.25** `src/Match.tsx:160–180` uses an optional remountDeps-derived key. The Logs route and `src/router.tsx:14–34` provide no remount key for search changes. **router-core 1.171.21** is the installed transitive version. These opened implementation paths explain why props change while state survives; this is not an Apollo response or backend filtering bug.

**Bounded fix:** give the hosted LogViewer an explicit URL-authoritative input plus a local input/committed-state distinction, or an equivalent narrowly scoped synchronization that distinguishes external navigation from acknowledgments of its own writes. Adopt destination filters/live before the destination's query/stream can use stale predicates. Preserve a newer local draft when an older own replace completes; invalidate pending old commits when an external destination arrives. A naive effect copying every prop or keying the entire viewer by every search string would clobber typing or discard readers on its own filter writes. Verify the chosen implementation against those races rather than assuming the datastore algorithm can be copied verbatim.

**Before settle:** keep existing service-type skeleton/hold and query-loading behavior. Once navigation commits, controls and request predicates must describe the destination; source-only rows can remain only under an explicit pending presentation, never as the settled result. An old network completion must not overwrite destination results.

## Scope and neighboring controls

Exhaustive production JSX search finds **one** LogViewer mount, `routes/services.$serviceId.logs.tsx:116`, serving web, private, worker and cron. Service base mapping is `features/services/lib/service-base.ts:19–23`. Static has no runtime LogViewer: `features/services/components/non-static-route.tsx:35–37` redirects it to canonical Events.

The distinct DatastoreLogViewer has **two** route mounts, Postgres `routes/databases.$databaseId.tsx:227` and Key Value `routes/keyvalue.$keyValueId.tsx:195`. Its `components/datastore-log-viewer.tsx:116–169` already adopts external URL text, reads instance directly and invalidates superseded text timers. This is a source-inspected good control, **not a live datastore replay this pass**. Deploy log panels have their independent local filters and are not this route.

Render-shaped aliases `/web`, `/pserv`, `/worker`, `/cron`, `/static`, `/d`, `/r` are enumerated in `common/lib/render-alias.ts:12–20,58–70`; suffix/query/hash are preserved. Service aliases reach the shared canonical route, with static ultimately gated. `t/r` query aliases retain existing canonicalization. No new aliases, new backend resampling or API shape changes.

REST GET /v1/logs, GraphQL logs and MCP list_logs retain existing explicit predicate semantics. The live subscribe endpoint retains its normal reader/filter behavior. Unauthorized, unavailable store, timeout, read failure and honest no-match remain their existing classes; this fix changes navigation-state ownership, not error taxonomy. Do not manufacture an empty success to hide an error.

## Prior guarantees and dedupe

This is an **uncovered residual**, not a claimed recurrence of previously verified mounted-navigation adoption. Source history points to `5e577a466` (w7/m42); its comment leaves adoption deferred. The five complete **w7/done/m42** DoD clauses remain required:

1. All filters round-trip/reload/copied links: text/type/off fresh loads and text write passed; clicking the clean link violates ongoing shareability. The other five structured filters and new-tab replay were not driven.
2. Minimal primary toolbar and five-filter disclosure: current type/text/range/live/Filters controls were seen; popover/count/chips were not exercised.
3. Every existing filter remains wired and store-only Live gating persists: the observed text/type history request is correct for local controls; the defect is its disagreement with destination URL. Full structured/SSE controls unprobed.
4. Render t/r aliases prefill and canonicalize: source inspected; alias live replay unprobed.
5. Existing range/debounce/history merge/empty/error behavior unchanged: normal literal search and fresh clean reload passed. Full range, paging, merge, forbidden/store-unavailable/timeout controls were not replayed here.

**w4/done/228** owns the repaired datastore URL omission, whose distinct adoption implementation was opened. **w4/m203** owns selection-time range clock; **w4/done/m151** owns routine-tick reader continuity; **w4/done/m192** owns pause/replay rows and deduplication; **w4/blocked/178** owns label discovery refresh; **w4/done/280** owns a paged-history caption. None owns the new mounted URL/predicate mismatch. Preserve their guarantees; this pass does not close or fully reverify those milestones.

Whole-board open/blocked/done searches for initialFilters, LogViewer, logFiltersFromSearch/logFiltersToSearch, URL state and Back/Forward, plus **57** non-done milestone titles, found no current owner. Four evidence READMEs accidentally matched the path glob and were excluded from the final milestone count. DO_NOT_DO has no matching exclusion. Latest 40 commits and targeted source history contain no landed adoption fix; current source matches the production symptom.

[Render logging documentation](https://render.com/docs/logging), opened 2026-10-13, describes runtime text/structured filtering and range/live controls. Exact authenticated Render sidebar and Back/Forward behavior remain unverified. This filing repairs bex's advertised URL shareability, not a claim Render must use bex's default encoding.

## Limits and cleanup

Native Back/Forward attempt returned no browser receipt; no result is claimed. Structured navigation, late URL completions, very rapid typing, other resource families, Chinese, hidden tabs and pending/error classes are implementation verification. Mobile Sidebar was manually dismissed; no separate sidebar-close defect is claimed. The first post-navigation query was not continuously observed; the captured later tick establishes persistent wrong predicates.

DeleteService returned true **08:45:37.774Z**. Own service and public URL both 404 at **08:46:03.742Z**; all seven inventory ID sets exactly match the baseline. Logout completed successfully; owners 401, zero cookies, one blank page at **08:46:27.845Z**. Exact owned session handles removed, global handle removed only on stat match. Settled console: zero errors/warnings. No foreign resource was changed.

Complete exact GraphQL request batches/response batches and safe UI receipts are in [evidence.json](evidence.json). Nanosecond timestamps remain strings. Supplementary screenshot and local ledger are ignored and not the sole support for any claim.
