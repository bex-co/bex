# A bulk move reports failure after its GraphQL batch outlives the response window

Why: a customer cannot trust the outcome of a normal multi-resource move when the dashboard reports both updates failed but their placements subsequently change.

**Severity:** major functional hosting defect. **Source:** continuous user-directed `qa-find-bugs w4`, a569/a570, 2026-10-15 UTC; QA sign-in through the canonical helper with muse.env. **Research main:** `f497ddab6`, synced before filing. This schedules a fix; no product code or tests are changed.

## Native reproduction and controls

The exact create requests, native move POSTs, complete successful responses, observable failed-response metadata, fresh state reads, capture driver and cleanup are in [evidence.json](evidence.json). No request was delayed, modified or automatically replayed.

1. Create the owned project `qa-20261015-a570-project` (`prj-db8b13rh673c73birsj0`) with Alpha `evm-db8b165eelgc73d2i33g` and Beta `evm-db8b16teelgc73d2i350`.
2. In Alpha, create Free BusyBox 1.37 web service `qa-20261015-a570-move` (`srv-db8b1djh673c73birsl0`), port 8886, and group `qa-20261015-a570-group` (`evg-db8b203h673c73birsn0`). Link only that service. Its synthetic `QA_A570_VALUE=MOVE_PRESERVED` reaches the public HTTP response. The group rollout is Live at `rev-2`.
3. On the project page, select those two rows, choose Beta and click Move once. Apollo sends **one HTTP POST containing two operations**, `SetEnvironmentServices` then `SetEnvironmentEnvGroups`. This desktop control succeeds in approximately **26.5s**: HTTP 200, correct response-origin headers, two successful result slots and both success toasts. Fresh reads show both memberships in Beta, Alpha empty and unchanged runtime.
4. Fresh-load Beta at **390×844**, select both rows, choose Alpha and click Move once. At **10:59:44.980Z**, CDP observes the same two-operation shape for Alpha. Approximately **31.0s** later the response is **502**, content type `application/problem+json; charset=utf-8`, server `cloudflare`, without `access-control-allow-origin`. Chromium records `net::ERR_FAILED` / `MissingAllowOriginHeader`. The dashboard displays both “Failed to update services” and “Failed to update environment groups” messages.
5. At **11:02:14.390Z**, an authoritative query shows **both** memberships in Alpha, Beta empty, the link retained, group revision unchanged, the original deploy still Live and the service still at `rev-2`. Its endpoint returns 200 `VALUE=MOVE_PRESERVED\n`. The exact individual commit times were not captured; do not claim both were complete at the instant the network failed.

The observer's raw callback offsets give **26,465ms** and **30,998ms** from captured request to response/failure. These are client observations, not per-resolver profiles. One earlier a569 fixture also produced committed moves with four CORS/failed-fetch console messages, but its native request bodies were not retained; a570 is the decisive fresh reproduction.

A fresh-session ordinary single inventory read and a two-operation read batch both returned 200. The passing native move itself carried the expected origin headers. Therefore a generally missing origin policy is not the observed mechanism, and the failure is timing-dependent rather than a demonstrated mobile-specific defect.

The viewed screenshot `.playwright-mcp/qa-a570-bulk-move-mobile-failure.png` shows the failed group toast and a busy intermediate source view with one selected row. It does **not** prove final partial placement or independently show both toasts. The durable capture records both messages; the later API query establishes the completed placements. The failed response body is inaccessible to page JavaScript, so no JSON error body is invented in the artifact.

## Producer, execution and consumer trace

- `dashboard/src/features/environments/components/environment-card.tsx:229–264` starts one setter for each selected resource kind and awaits their promises. There are **four** kinds: service, database, Key Value and group. Both this component and `manage-resources-dialog.tsx` consume the four setter hooks: **two** production UI consumers, with the latter source-scoped only.
- The browser has **one** `new BatchHttpLink` constructor, in `common/apollo/factory.client.ts:28–46`: operations without an independent signal share a 10ms batching window, up to 32 members. Signal-bearing operations use HttpLink; SSR has its separate single-operation HttpLink in `factory.server.ts:78–92`.
- Actual pinned **Apollo 4.1.3** was opened. `link/batch-http/BaseBatchHttpLink.js:68–116` serializes the operation list into one fetch. `link/batch/batching.js:94–137` distributes indexed results to their observers, but fans one transport error to **every** observer. That explains two failure toasts from one lost response; this is not proof that two writes were refused. `common/lib/graphql-error.ts:146–149` correctly uses generic mutation copy for a transport error without a GraphQL answer.
- There is **one** registered batch endpoint, `POST /graphql` (`lego/backend/internal/api/server.go:1277`). The handler has **two** production `executeGraphQL` call sites: the array loop and the single-document path (`:1487,1504`).
- The array loop executes members **serially** and writes the result array only after all members finish (`:1485–1496`). Each call creates a fresh timeout at `:1543`. `graphql_cost.go:28–39` sets each document's budget to 25s, while `cmd/api/main.go:1442–1448` gives the HTTP response 30s. Two individually bounded operations can therefore consume more time than their shared response permits.
- Go 1.26's actual net/http source was opened: `server.go:986–989` sets the connection's write deadline for the request, and `:3004–3009` defines it for response writes. It is not restarted for each GraphQL document. The Dockerfile pins the Go 1.26 build family by digest; a local 1.26.8 source was read, not the deployed patch version.
- The resulting budget mismatch is established in code and fits the approximately 31s live failure. Origin logs and individual resolver timings were not inspected, so the exact low-level socket failure is an attribution from source and timing rather than an origin trace.

## Bounded correction and expressible outcome

Give the accepted array **one shared execution deadline**, retaining the current margin below HTTPWriteTimeout. Each member inherits the remaining budget. Keep sequential execution and one ordered result slot per input; do not raise HTTP timeouts, change origin policy or replay the writes.

A completed prefix retains its actual results. A dispatched write whose result is uncertain retains `WRITE_OUTCOME_UNKNOWN` and read-before-retry guidance. An undispatched later slot gets **`OPERATION_NOT_STARTED`**, with copy stating that this operation did not start; earlier operations may still have committed. Check dispatch immediately before entering execution.

This gate matters: the actual **graphql-go v0.8.1** `executor.go:52–94` starts a worker goroutine before selecting on ctx.Done. Calling it with an already-expired context does not establish that no resolver starts. The skipped-suffix scenario is source-derived verification work, not a production observation.

The existing layer can represent the target: `graphql.Result` has nullable interface Data and an error slice (`types.go:10–14`); `gqlerrors.FormattedError` has an open extension map; the API's `graphqlErrorResult` already preserves coded extensions (`server.go:1558–1567`). Preserve response cardinality and telemetry classification when constructing unstarted slots. No new schema field is required.

The browser consumer can read the new message: `mutationErrorMessage` accepts structured CombinedGraphQLErrors without a closed code allowlist. `retry-link.ts:shouldRetry` excludes mutations, and GraphQL execution errors are results rather than transient transport errors. Preserve that behavior. Group assignment's current uncertain-outcome placement refresh and canonical-selection reconciliation remain required.

Bad input, not-found, forbidden, unauthenticated, unavailable and ordinary conflicts keep their established distinctions when processed. Preserve single-query QUERY_TIMEOUT, dispatched-mutation WRITE_OUTCOME_UNKNOWN, empty-document neighbors, batch cap and one-request admission. The new batch-specific correction must not normalize an unknown write into an unstarted write or imply rollback.

## Blast radius, parity and verification limits

The API change applies to JSON-array requests at that one endpoint. The service setter covers web/static/cron/private/worker; Postgres, Key Value and groups have their own setters. Only Free web + group was live-replayed. Other clients sending arrays are source-scoped; REST and MCP direct feature calls and single-document HTTP requests retain their existing contracts.

The observed UI route is `/project/$projectId` with its environment query parameter. Route enumeration found no separate `/p/$` project alias. Resource-detail aliases are separate consumers and are not claimed live-tested by this finding.

Require real HTTP deadline tests with shortened deterministic budgets, completed-prefix preservation, an uncertain current write and a third resolver that never starts. Keep fast ordered batches, single-document timeouts, query-batch admission, SSR/signal-bearing requests, mixed empty-query results and no mutation retry as controls. These are implementation tasks, not tests run for this filing.

[Render's Projects documentation](https://render.com/docs/projects) describes bulk service moves and group placement. It promises no GraphQL batch deadline or atomicity contract. [Apollo's BatchHttpLink reference](https://www.apollographql.com/docs/react/api/link/apollo-link-batch-http) documents batching; the mechanism here was checked in the pinned source and the native request. No signed-in Render replay or exact Render timing comparison is claimed.

## Dedupe and prior guarantees

Searched all **9,156** board Markdown files across open/blocked/done states for batch/deadline/timeout and membership terms; inspected **72** actual open milestone titles, DO_NOT_DO, the latest 40 product commits and targeted execution-budget/batch history. Main remains on the per-document loop; no matching undeployed batch-budget fix exists.

- **w4/done/m140:** its corrected single-document 25s/30s relation is intact. This is the unbounded **sum across documents**, not a re-observation of its original single 7-day log query. Its original 7-day API budget/CORS, prompt Logs timeout UI and exact 1h controls were not re-run here and remain regression requirements.
- **w4/blocked/m206:** it owns scoped create/name recovery, a two-group move inside **one** mutation and post-interruption placement reconciliation. This filing concerns **two distinct operations in one HTTP envelope**, even when each has its own valid budget. Original empty scoped-create, reservation residue and two-unlinked-group cases were not re-run. The populated linked create passed; the mixed timeout view reconciled from an intermediate selected row to empty source. These are related variant observations, not full m206 closeout. Its old residue was not touched.
- **w4/done/m100:** batching/admission and Metrics fan-out remain constraints; its concurrency, degraded-state and deploy-commit DoD was not replayed. Do not remove query batching to hide this failure.
- **w4/done/263:** successful membership publication differs from a lost HTTP response. **w4/blocked/m204**, **done/223** and **done/256** concern filter navigation, labels and log payloads respectively, not this deadline.
- Prior w9/m89/m92 hunt shapes and repository anti-goals introduce no matching fix or prohibition.

A separate milestone is justified because this request-lifecycle correction and unstarted-operation contract span every GraphQL batch, beyond the existing group-store recovery task. No original milestone is reopened or marked complete by this filing.

## Cleanup

DeleteService and DeleteEnvGroup succeeded; the project was deleted only after both exact environments and its resource arrays were confirmed empty. At **11:04:33.262Z**, all five immutable-ID REST lookups and the public URL returned 404 and all seven baseline ID sets matched.

Exact helper logout returned exit 0 / `ok logged-out`. At **11:05:22.922Z**, whoami returned 401, cookies were zero and the browser had one blank page. Only this session's saved handle was removed. Physical cluster reclamation was not inspected.
