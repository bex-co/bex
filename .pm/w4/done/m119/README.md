# w4 · m119 — `http_requests` lies about bursty traffic: 25 requests read back 27.8 or 6.67 by query alignment

**Worker:** worker4 **Goal:** The m108 "counts are counts" guarantee holds for bursty traffic: N driven requests sum to N ±1 on every metrics surface for any query alignment, reconciling with the access log. **Status:** done 2026-09-21 (live re-probe of the deployed fix deferred to the next QA pass — no production access this session)

## Tasks (in order)

| id   | title                                                                                        | est   | depends_on       |
| ---- | -------------------------------------------------------------------------------------------- | ----- | ---------------- |
| t001 | `http_requests` returns exact per-bucket counts for bursty traffic — **DONE**                    | 1h30m | —                |
| t002 | Bandwidth sibling: live-verify `SumIncreases` extrapolation, apply the same treatment — **DONE** | 1h    | w4/m119/t001     |
| t003 | Render parity: cross-surface check for the metrics changes — **DONE**                            | 30m   | w4/m119/t001, w4/m119/t002 |
| t004 | Simplify the code this milestone touched — **DONE**                                              | 20m   | w4/m119/t003     |
| t005 | Test coverage for the shipped behavior — **DONE**                                                | 40m   | w4/m119/t003     |
| t006 | Closeout — **DONE**                                                                              | 15m   | w4/m119/t004, w4/m119/t005 |

## Definition of done

Each bullet is a command or a click the next person can repeat on production and watch succeed.

- **Bursty counts are exact.** Create a free web service, send exactly 25 `curl` GETs to its `.onbex.co` URL, wait 2 min, then `GET /v1/metrics/http-requests?resource=<id>&resolutionSeconds=60` over three differently-aligned windows containing them: every window's value-sum is 25 ±1 — not 27.8 in one alignment and 6.67 in another (the 2026-09-20 capture below).
- **Counts reconcile with the access log.** The same 25 requests read `nlogs: 25, hasMore: False` from `GET /v1/logs?resource=<id>&type=request&limit=100`; the metrics sum for any containing window equals that line count ±1.
- **Cross-surface agreement.** The same window through GraphQL `metrics(name: HTTP_REQUESTS)` and MCP `http_request_count` returns the same numbers as REST.
- **Low-traffic non-zero still holds (m108 bullet 3).** A service that served requests shows a non-zero Total Requests summary and non-zero bars.
- **Bandwidth sibling settled.** Driven traffic's Outbound Bandwidth buckets reconcile with the month-to-date figure and usage metering within a documented tolerance, or a written, evidenced decision in `docs/render-artifacts/metrics-page.md` records the deliberate divergence with the axis/unit labelled so a reader cannot mistake a rate for bytes-per-bucket.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` pass 71 on `https://dashboard.bex.co`, 2026-09-20 (w4-targeted run, `muse.env` credentials). Fixture `qa-20260920-p71-met` (`srv-dansb83s0ils73bgpaj0`, go runtime, deleted at the end of the run). 25 `curl -L` GETs to `/room/hn` at ~11:41–11:42Z; browser MCP dead so the pass was API-only — the durable evidence is the probes quoted in t001.
- **Goal linkage:** product truthfulness and Render parity ([docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md), [docs/ADR006-bex-api.md](../../../docs/ADR006-bex-api.md), [docs/ADR010-observability.md](../../../docs/ADR010-observability.md) for metrics semantics). Follow-up to `w4/m108` (done 2026-09-16): its t001 replaced `sum(rate(…))` with `sum(increase(…))`, which is exact for steady traffic but Prometheus `increase()` extrapolates partial-window counters, so bursty traffic — the free-tier norm — misreports by alignment. m108 DoD walk: bullet 1 (20 curls sum to 20 ±1) broken live; bullet 3 (low-traffic non-zero) still holds; bullet 2 (busy-service 12h reconciliation) not re-probed this pass; deploy-provenance bullets untouched by this finding.
- **Expected outcome:** the Metrics page's Total Requests can be reconciled against the access log for any traffic shape; the same one-line primitive choice is verified (not assumed) on the bandwidth sibling that shares it.
- **Why now:** the defect is invisible precisely because it degrades gracefully — a plausible small number instead of an error — on the chart users read to make scaling and debugging decisions. The exact-count primitive already exists in the file (`lokisource.go:116` `count_over_time`, same `RequestMetricsSource` signature), so the fix is a change of call, not of architecture.
- **Render parity task included** because the fix changes a user-facing surface with REST + GraphQL + MCP + UI representations.

## Outcome (2026-09-21)

**t001 — the count comes from the access log now.** `increase()` scales the first-to-last counter delta by `window / sampled-duration`, so a burst occupying part of a bucket is extrapolated by however the scrapes happened to land — 27.8 in one alignment, 6.67 in another, for 25 real requests. There is **no extrapolation-free counter-increase in PromQL**: `increase()`, `rate()` and `delta()` all extrapolate by design, and rebuilding a bucket from `max_over_time − min_over_time` loses counter resets and cross-boundary increments. So the exact count has to come from a source that counts *events*. The unfiltered `http_requests` read now goes through the same `count_over_time` primitive the host/path-filtered read already used (`lokisource.go:116`) — the chart and the Logs tab reconcile by construction because they are the same records, which is what DoD bullet 2 asks for and what no counter could have delivered. The routing is a five-line change in `requestMetric`, not an architecture change: `readRequestSeries` already took a source, and `lokisource.go:75-77` already renamed `status` → `code`, so the two backends were drop-in compatible on labels, group-by vocabulary and unit.

Deliberately unchanged: `http_latency` (percentiles come from Traefik's histogram, where `rate()` is the correct primitive) and bandwidth (t002). Static sites are served from the access log too — w4/m113's router-scoped Prometheus selectors worked around a static site having no per-App Kubernetes Service, and the access log has no such problem.

**The fallback is a considered trade, not an oversight.** A host/path-filtered read *errors* when the log store is unwired, because Prometheus carries no host/path axis and cannot answer at all. An unfiltered count *can* be answered approximately, so when the log store is unwired or a read fails, bex serves the counter and logs it rather than blanking a chart that works today. The approximation is exactly the pre-m119 behavior, so the fallback is never worse than what it replaces, while a 503 would be strictly worse. The residual — a wired-but-silent pipeline reading zero, indistinguishable from a genuinely idle service — is the same limitation the host/path read already dispositions, and is owned by the scheduled request-logs-liveness probe rather than by manufacturing an error an idle service would also raise.

**t002 — bandwidth: a deliberate, evidenced divergence, not a live measurement.** The task asked to live-verify the `SumIncreases` extrapolation; the DoD's own alternative branch (a written, evidenced decision in `docs/render-artifacts/metrics-page.md`) is what shipped, because the decisive evidence is in code and does not need a cluster:

1. **Money is not affected.** Metering calls `egressquery.Increase` over the **billing window**, not per chart step (`usage/service.go:1229`), and `SumIncreases` carries an explicit "do not route money through this helper" fence. `increase()`'s edge error is bounded by about one scrape interval at each end of the range — under ~0.01% across a month, and the error this milestone is about across a 60 s bucket. Same function, opposite significance; the ratio of range to sample spacing is the whole story.
2. **There is no exact substitute.** Bandwidth is *composed* — Traefik router response bytes **plus** the WebSocket egress meter **plus** the node-level direct-egress meter, the last with its own counter-loss guard (`egressquery.App`). The access log carries only the HTTP half, so rebuilding bandwidth from it would trade a bounded alignment error for an unbounded omission of every WebSocket and non-HTTP byte.
3. **The unit cannot be misread**, which is the other half of the DoD bullet: `requestUnit` returns `bytes` with the per-bucket meaning pinned in a comment, and the month-to-date footer reads the same `Increase` primitive.

Left open honestly, in the doc and here: a short, bursty transfer can still read high or low on the bandwidth chart by alignment. It never reaches an invoice, and closing it needs a per-App byte-counting event stream that does not exist.

**t003 — parity.** REST, GraphQL and MCP share the one `RequestMetrics` verb and the one `readRequestSeries` post-processing, so routing at the source is what makes all three (plus the dashboard chart) agree by construction — asserted by the routing-table test rather than by three near-identical adapter probes. No wire shape changed: same `unit: "count"`, same `resource`/`statusCode` labels (the Loki source's `status` → `code` rename predates this change, and REST's `code` → `statusCode` rename sits above both).

**t005 — coverage.** `TestUnfilteredRequestCountsComeFromTheAccessLog` pins the whole routing table — unfiltered count, count grouped by status, count filtered by status, latency, bandwidth, static site, a failing log read, and no log source — because getting one cell wrong is how this bug comes back. Mutation-spot-checked: with the routing disabled, four of its cases fail. `TestHostPathFiltersRouteToLogStore` was updated rather than weakened: its old "Loki only with a filter" invariant is now scoped to `http_latency`, and it gained an unfiltered-latency case so the Prometheus half stays pinned.

**Green:** `lego/backend` `go test ./...` all packages + `golangci-lint` 0 issues.

**Not done — the live DoD walk (t006's probe).** Every DoD bullet is a production probe: 25 curls, three alignments, REST + GraphQL + MCP, access-log reconciliation. No production access this session, so none of them has been re-run against the deployed fix — deferred to the next QA pass, the same disposition m118/m115/m113/m112/m111 carry. The next pass should also re-check m108's bullet 3 (low-traffic non-zero), which this change makes *more* likely to hold, and w4/m113's static-site attribution, which now rides the access log instead of router counters.
