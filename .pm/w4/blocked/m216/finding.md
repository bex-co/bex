# Deploy log search changes Greek final-sigma matches

Why: a customer searching the same loaded application record receives different answers in service Logs and deploy details.

**Severity:** minor. Functional live QA **a743**, 2026-10-16 UTC, workspace `tea-d98210cbbpdc73dcrkvg`; source synchronized to **f317cd712**. This files work only. No product implementation or deployment changed.

## Reproduction and exact target

Create an owned Free web service using `node:24-alpine`, port8080, and the exact native CreateService / SetStartCommand requests in [capture.json](evidence/capture.json). The revised command emits these four plain, escape-free records three seconds after startup and again on GET `/emit`:

```text
QA_A743 ΟΣ
QA_A743 ος
QA_A743 οσ
QA_A743 Alpha
```

Owned service **srv-db94lthkoksc73a7vi10**, name **qa-20261016-a743-case**. The initial deploy was **dep-db94lthkoksc73a7vi1g**; the native command edit created **dep-db94p8ffsf9s73epsggg**, which reached Live / revision2. External GET `/emit` returned HTTP200 and `QA_A743 ready\n`. The first immediate-startup sample lacked its first record, so it was excluded; the complete second-deploy input is independently captured.

1. Open `/services/OWN_ID/logs?type=app&live=0`. The complete input is visible. Search `QA_A743 οσ`: native Apollo Logs returns uppercase `ΟΣ` and normal lowercase `οσ`. Search `QA_A743 ος`: it returns final-sigma `ος` alone.
2. Open the second deploy detail. Its unfiltered application leg returns exactly the four records above, with `hasMore:false`, in **16:11:13.244694Z–16:11:24.82011Z**. Search `QA_A743 οσ`: only `οσ` remains; uppercase `ΟΣ` is incorrectly omitted.
3. Search `QA_A743 ος`: uppercase `ΟΣ` is incorrectly included with `ος`.
4. Fresh-load the deploy at390px and repeat: the same failure appears. The scoped log-panel screenshot was opened and inspected; its accessibility tree, input and sole result are retained in capture.json. The document is390px wide with no overflow and no captured console errors.
5. The same-window direct GraphQL controls, copied from the native deploy query and adding only `text` / `textMode:"display"`, return **2 / 1 / 1** original records for normal sigma / final sigma / ASCII `qa_a743 alpha`. All three return200, no errors, `hasMore:false`. The native deploy ASCII control returns its one correct `Alpha` record. These are complete exact requests and responses in capture.json.

**Required target:** preserve the backend's existing **simple Unicode lowercase followed by literal substring** contract. For this fixed four-record window, normal-sigma search returns `ΟΣ` and `οσ`; final-sigma search returns `ος` alone; ASCII returns `Alpha`. Normalize both needle and displayed haystack using the same per-code-point rules. Preserve case distinctions that Go's existing matcher preserves. Do not change backend/CLI semantics to full case folding, contextual or locale-dependent conversion, regexp search, or normalize both views onto the incorrect deploy results.

## Root cause and concrete repair

- `dashboard/src/features/deploys/components/deploy-log-panel.tsx:133–140` calls whole-string JavaScript `toLowerCase()` on both the search and `stripAnsi(l.message)`. ECMAScript default case conversion includes contextual final-sigma conversion. In the actual **Chromium155.0.8059.84** runtime, `"ΟΣ".toLowerCase()` is `"ος"`. This is the language operation itself, not an inferred Apollo/React behavior.
- `lego/backend/internal/logs/service.go:332–341,477–486` normalizes terms and messages with Go `strings.ToLower` before literal Contains. The actual local **Go1.26.5 / Unicode15.0.0** standard library `strings.go:727–764` delegates non-ASCII strings to `Map(unicode.ToLower,s)`; `unicode/letter.go:279–286` maps one rune at a time. `ΟΣ` becomes `οσ`, while `ος` retains its final sigma. Native service responses and exact-window API controls exhibit these semantics.
- The consumer at `deploy-log-panel.tsx:189–199` passes the already filtered array to LogLineList; dropped rows cannot be restored by rendering. `use-deploy-logs.ts:89–105` loads the existing typed, bounded windows without server text search. Its actual unfiltered app response contains all four lines, so this is neither ingestion loss, pagination nor a service/deploy time-window difference.
- GraphQL `logs.text` / `textMode` are optional strings (`logs/graphql.go:56–78`); messages already carry the needed original text. A client matcher can express the repair without a schema change or new API round trip.
- Implement a narrow display-search normalizer under `dashboard/src/features/logs/lib/`, using a compact committed simple-lowercase map/ranges tied to the backend's Unicode table, with a reproducible Go-derived fixture/drift check. Apply it to both needle and stripAnsi output in this panel. The captured model uses the actual1433 Go mappings and produces the required **2 / 1 / 1** results; it is a proposal model, not a shipped fix.
- Merely switching to `Array.from(s,c=>c.toLowerCase()).join("")` fixes contextual sigma but still expands `İ` into i plus combining dot, where Go produces i. Comparing all **1,112,064 Unicode scalar values** in this browser against the local Go table finds **55 single-code-point differences**, including that expansion and newer browser mappings. The complete comparison and Go probe source/table are durable below. This is a runtime/model compatibility bound, not55 live UI reproductions. Avoid hand-maintaining a sigma-only or browser-version-specific exception list.
- Preserve no-search/type/instance predicates, debounce and loading/error frames, limited-window/paging notices, original messages, record keys/timestamps and ANSI rendering. Before debounce settles, retain the current input/result behavior; no extra loading phase is needed for a synchronous normalizer. Do not change the ANSI parser owned by [327](../../327.md).

## Consumers and adjacent behavior

Exhaustive production-source grep finds **one DeployLogPanel caller**, `deploy-detail-page.tsx:136`, and **two route entrypoints**: `routes/services.$serviceId.deploys.$deployId.tsx:29` and `routes/static.$serviceId.deploys.$deployId.tsx:24`. Both share this local filter. The service route covers web, worker, private and cron; static has the second route. Postgres and Key Value use DatastoreLogViewer and server queries, without this deploy panel. Source tracing covers this seven-type family; only the owned web application leg was reproduced live.

Keep the fix local to deploy displayed-text matching. Service LogViewer and Postgres/Key Value server matching, REST/GraphQL/MCP, raw defaults and build followers retain their contracts. Do not globally replace other dashboard lowercase searches. Unauthenticated, forbidden, missing, unavailable and timeout handling stay with existing readers; a failed/denied read must never become a client fallback or successful empty result.

## Dedupe and boundaries

Searched **9457** board Markdown files across active/blocked/done paths with sigma, Unicode, lowercase, case-insensitive and case-folding terms; inspected **81 actual open/blocked milestone titles** (two evidence directories were excluded from that count), DO_NOT_DO, the latest40 dashboard/Go commits and targeted panel history, then synchronized main. No client sigma filing or fix found. The predicate dates to **4bfbd0647**, rather than a recent regression.

- [w8/done/m58](../../../w8/done/m58/README.md) corrected backend Loki candidate selection and raw scan coverage. Its DoD, scope and complete table preserve normalized literal semantics, especially dotted-İ; dashboard UI coverage was expressly unexercised. The current same-window backend controls pass. This is a separately traced client residual, not a re-file of Loki/pagination work.
- [w4/done/m189](../../done/m189/README.md) introduced server display search for ANSI-crossing substrings. Its recorded deploy passing controls did not exercise contextual Unicode lowercasing. All a743 records are plain and contain no ESC/CR.
- [327](../../327.md) concerns supplementary-code-point CR overwrite rendering. This fixture has no carriage returns, styled spans or rendering discrepancy.
- w2/done/m167 concerns server literal term handling; w5/blocked/m118 concerns naming/config-key rules, not this loaded-log predicate. Existing log chip, reader refresh and truncation items do not own it.

[Render logging documentation](https://render.com/docs/logging#log-filters) describes searching logs and separate deploy logs, but does not specify this Unicode normalization choice. No authenticated Render sigma comparison was performed. The target is the established Bex contract in [ADR010](../../../../docs/ADR010-observability.md#log-filters), including its literal-search divergence from Render regex/wildcards, and the proven passing backend controls. [ECMAScript toLowerCase](https://tc39.es/ecma262/multipage/text-processing.html#sec-string.prototype.tolowercase) supplies the actual contextual-conversion mechanism.

**Unverified:** live static/build/pre-deploy logs, worker/private/cron, Postgres/Key Value, REST/MCP/native CLI and authenticated Render for this sample; other browsers; deployed compiler/image SHA. Additional Unicode mappings and the proposed correction were checked in a local/browser model, not55 new production probes.

## Cleanup and durable evidence

The exact native DeleteService returnedtrue at **16:15:06.997Z**. Parent and both deploy paths return404; all seven original sorted resource ID sets match at **16:16:06.464Z**. Exact owned App/pod selector is empty at **16:16:08.747867Z**; public URL returns404 again at **16:18:25.480542Z**. No paid operation occurred.

The successful a743 session was explicitly logged out. Its exact retained jar subsequently returns401; the recovered browser clear at **16:17:33.394Z** confirms about:blank,401,zero cookies and one page. Exact local handles were removed. An earlier failed login helper installed no jar and created no fixture; its failure phase is unverified. Logout's first browser-clear navigation timed out after an auth redirect; that was recovered, not filed as a product issue. No bulk/historical session revocation was performed.

[Complete durable capture](evidence/capture.json) contains accepted fixture writes, unfiltered native app data, fresh desktop/mobile DOM, exact-window query/response controls, actual browser primitive proof, proposed-model qualification and cleanup. [Go probe source](evidence/lowercase-probe.go.txt), [actual Go table](evidence/go-lowercase.json) and [complete scalar comparison](evidence/browser-lowercase-comparison.json) make the compatibility evidence reproducible. Local inspected image: `.playwright-mcp/qa-a743-sigma-panel-mobile.png`; durable text/API evidence does not depend on ignored screenshots.
