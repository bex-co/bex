# w5 · m503 — GET /v1/logs answers in a few seconds, not 10–30 s for one read in four

**Worker:** worker5 **Goal:** reading a service's logs over the API takes a few seconds for the query shapes callers actually send, instead of 10–30 s for a quarter of reads and a timeout for the slowest. **Status:** done (t001 to t006 shipped in `31108e539`; t007's 24 h production reading 2026-11-20, see § Findings)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Find the slow query shapes: time the LogQL the handler builds against production Loki, per window, type, filter and direction — **DONE** | 1h30m | — |
| [t002](done/t002.md) | Fix the dominant causes — **DONE** | 2h | t001 |
| [t003](done/t003.md) | Finer latency buckets, query-shape labels, and a latency alert with its panel — **DONE** | 1h | t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 15m | t002, t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004, t005 |
| [t007](done/t007.md) | Closeout (24 h production reading) — **DONE** | 15m | t006 |

Total: **7 tasks, about 5h45m**.

## Definition of done

1. The README's Findings record each measured query shape's time against production Loki and the cause of the 10–30 s requests.
2. Over 24 hours of production traffic after the fix deploys, `GET /v1/logs` p95 is under 5 s, measured with buckets at 15, 20 and 25 s, and no request reaches the 28 s REST deadline for the shapes in the Findings (or the remaining shapes are named with their cause and a follow-up item).
3. MCP `list_logs` and REST `GET /v1/logs` still answer the same bytes for the same query (w4/m378), and Render's paging fields (`hasMore`, `nextStartTime`, `nextEndTime`) keep their meaning.
4. A latency alert for the route has a panel, and `bash scripts/obs-coverage-check.sh` passes.
5. Regression tests cover the fix.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5`, round 10 item 3 (2026-11-18), selected with `/pmx them all to w5`.
- **Evidence (production Prometheus, 7 days to 2026-11-19):** 9,096 `GET /v1/logs` requests; 3,916 finished under 1 s and 2,262 took between 10 and 30 s. In the last 24 hours, 20 answered 500. The p99 (28.7 s) sits at the 28 s REST deadline w5/m286 set. The histogram has no bucket between 10 and 30 s, so its p90 and p95 (about 17 s and 24 s) are interpolated. Traffic is bursty (1,037 requests in the three hours from 2026-11-18 15:00Z). CLI telemetry shows `bex logs` runs at p95 1.3 s over 862 runs, so the slow shapes come from other API callers: the mobile app's REST transport (`mobile/src/features/logs/rest-transport.ts`) and verification scripts (`scripts/logs-verify.sh`, `scripts/cli-logs-verify.sh`) use this route.
- **Code:** `lego/backend/internal/logs/loki.go`, `loki_streams.go` (one `query_range` per read), `rest.go`, `render.go` (paging).
- **Goal linkage:** hosting observability on Render's API ([ADR018](../../../../docs/ADR018-render-parity.md) Logs rows, [ADR010](../../../../docs/ADR010-observability.md)).
- **Expected outcome:** logs reads stop timing out on wide windows, for the mobile app and API clients alike.
- **Why now:** the slowest reads fail at the deadline instead of finishing, and one read in four takes 10–30 s.
- **Render parity:** included (t004). REST, GraphQL and MCP share the logs service, so a change to how it queries Loki must leave all three identical and Render-shaped.

## Findings

### 2026-11-19 — t001: the slow shapes, timed against production Loki (loopx w5, read-only)

**Who sends the slow reads.** The edge's access log for bex-api (Loki, the `bex-system` request streams; the edge drops query strings, so the shapes were found by timing them) holds 11,827 `GET /v1/logs` reads in the 7 days to 2026-11-19, 1,072 of them 10 s or longer. 1,069 of those came from one Render CLI 2.27.0 client on macOS reading about once a second with several reads in flight (9,130 of the week's reads); browsers sent 2,007 with 3 slow, curl 654 with none. The slow reads stop where a budget stops them: 294 took 10–15 s, 764 took 15–20 s (the 18 s scan budget of a `text`, `path` or `host` search) and 11 took 20–25 s, all answered 200. The 26 answers of 500 (2026-11-18 17:25–18:00) were Loki answering 500 within 1.6 s (`bex-api: internal error: loki: status 500`) while the per-stream fan-out below ran; Loki's own log for that hour is no longer kept, so the cause inside Loki is not known.

**How it was timed.** `lego/backend/internal/logs/loki_timing_test.go` (build tag `lokitiming`; targets from the environment, never committed) reads through `NewLokiSource` as `QueryLogs` hands it a query (Hobby floor, the 101-line probe page, the App-id scope), against production Loki through `kubectl port-forward`, on the busiest service (about 2.5 million lines a week, about 20,000 request lines an hour, 24 request streams in an hour and 153 in a week) and an idle one (about 17,000 lines a week). `limit` 1000 is the same read as 100: the API clamps it (`maxLogLimit`). The port-forward adds about 0.17 s per sequential request and carries 3–5 MB/s, so its wall time overstates the cluster's; requests, Loki's own `execTime` (summed over its pieces) and MB are the comparable numbers. Loki runs as one binary limited to 1 CPU and 1 GiB with 4 querier workers; over the week it used 0.012 cores at the median and 0.78 at the peak.

**Before → after**, as requests / Loki s / MB / wall s (lines); `default` is no `startTime` (seven days), `partial` a page the 18 s budget cut, with how far back or forward it reached:

| Service | Shape | Before | After |
| --- | --- | --- | --- |
| busy | 1 h, all types, `text`, backward | 216 / 1.99 / 23.5 / 16.1 (0) | 17 / 0.42 / 12.2 / 4.1 (0) |
| busy | 1 h, request, `text`, forward | 211 / 1.77 / 23.4 / 15.8 (0) | 17 / 0.42 / 12.2 / 3.7 (0) |
| busy | 24 h, all types, `text`, backward | 245 / 2.08 / 25.4 / 18 (partial, 1 h 1 m) | 52 / 1.41 / 39.5 / 18 (partial, 3 h 5 m) |
| busy | 1 h, all types, none, forward | 23 / 0.22 / 0.3 / 1.3 (101) | 2 / 0.02 / 0.2 / 0.4 (101) |
| busy | 24 h, all types, `statusCode=5xx` | 11 / 1.46 / 0.2 / 1.2 (101) | 9 / 0.13 / 0.3 / 4.9 (101) |
| busy | default, `type=app`, none | 6 / 5.97 / 0.7 / 16.5 (101) | 6 / 5.04 / 0.8 / 9.1 (101) |
| busy | default, all types, `path`, backward | 8 / 13.16 / 0.1 / 18 (partial, 127 h) | 8 / 13.35 / 0.1 / 18 (partial, 127 h) |
| idle | 24 h, all types, `text`, forward | 48 / 14.28 / 4.1 / 5.7 (0) | 21 / 0.50 / 4.8 / 6.5 (0) |
| idle | default, all types, `text`, backward | 42 / 28.84 / 5.9 / 12.8 (0) | 38 / 2.32 / 9.4 / 16.3 (0) |
| idle | default, all types, none, forward | 12 / 65.02 / 0.2 / 13.0 (101) | 28 / 2.36 / 0.6 / 7.8 (101) |
| idle | default, request, none, forward | 12 / 63.16 / 0.2 / 18 (`QUERY_TIMEOUT`) | 28 / 2.45 / 0.6 / 7.4 (101) |

Shapes whose page is short or reads one stream (most `app`, `build` and `level` shapes over an hour, any read of an idle hour) were one or two requests under 0.1 s of Loki time before and after.

**The causes, in order of cost:**

1. **A full page was read again once per stream (w21/063), and a text search fills a page every 1,000 raw lines.** Every full page listed the window's streams and read each alone. On the busiest service that is about 22 requests per 1,000 raw lines, so a one-hour text search spent 216 requests and most of its budget. On the idle service a forward page whose lines spread over days read every stream across every 15-minute piece of those days: 63 to 65 s of Loki time, 13 s or a `QUERY_TIMEOUT`.
2. **Those fan-outs queue on a small Loki.** Four querier workers on one CPU serve every reader, so one fan-out of a few dozen stream reads, each cut into pieces, holds them while the client's next overlapping read waits.
3. **A text search reads every raw line into bex-api** (the match is in Go, `lokiTextScanPage`): about 0.4 s of Loki time and 12 MB per hour of the busiest service even when each line is read once, so a search over a day or a week of it still ends at the budget: [w5/198](../../done/198.md).
4. **The default window is seven days, where Render's is an hour.** A sparse type's default read walks a week of pieces before it has a page (`type=app` on the busiest service, 5 s of Loki time, unchanged): [w5/197](../../done/197.md).
5. **A `path` or `host` filter makes Loki parse every request line's JSON:** a week of the busiest service is 13 s of Loki time and a partial page at the budget. Unchanged here: the filter runs inside Loki and the cost is Loki's.

### 2026-11-19 — t002 to t006 (loopx w5)

- **t002, the fix (`lego/backend/internal/logs/loki_streams.go`, `loki.go`).** The merge that loses lines runs only on a read Loki's frontend cut, and a window inside one cut (a 15-minute bucket counted from the Unix epoch, `pkg/util.ForInterval` in the pinned 3.6.7) goes to the querier whole. So a full page across a cut is narrowed to its far line and read again in parts from the end the direction keeps: one bucket, then twice the last part's buckets each time, until the parts hold a page. A part inside one bucket or short of a page is exact; a full part across a cut holds the rest of the page and is read the same way. No stream listing is left, so the 256-stream refusal is gone with it. A text search that fills a raw page then reads on inside the bucket its edge is in, until a whole bucket holds under half a raw page. A 5,000-line raw page was tried and dropped: it cut requests but not Loki time or bytes, and 5,000 long lines could pass the 64 MiB response bound.
- **t002, what is left above 5 s:** a `text` search over more than a few hours of the busiest service (bound by the raw bytes it reads, w5/198), a default-window read of a type the service logs little of (w5/197), and a `path` or `host` filter over days of a busy service (Loki's JSON parse). Every other shape timed here is a few requests and under 1 s of Loki time.
- **t003, measured and watched.** bex-api's request histograms gain 15, 20 and 25 s buckets (`core.RequestDurationBuckets`); `bex_api_logs_page_duration_seconds{window,type,filter,direction,result}` times every logs page on REST, GraphQL, MCP and project views by shape; `BexApiLogsReadSlow` warns when more than 5% of `GET /v1/logs` reads take over 5 s for 15 minutes with at least 20 reads in 30 minutes (`deploy/gitops/base/rules/logs_read_alerts_test.yml`); panels 41 and 42 on `bex-api-gateways`. `scripts/obs-coverage-check.sh` and `scripts/gitops-validate.sh` pass.
- **t004, Render parity.** REST, GraphQL and MCP share `queryLogPage` and `NewLokiSource`, so they still answer the same lines (`TestMCPListLogsIsRESTsBody` and the logs suites pass), and `hasMore`, `nextStartTime` and `nextEndTime` keep their meaning: the reader returns the same exact page in fewer reads. One drift from Render's contract, filed as w5/197: with no `startTime` Render reads the last hour and bex the last seven days.
- **t005, `/simplify`:** the outward reader counts buckets as an integer rather than a `time.Duration`, its cost comment matches the measured bound, and the request histograms read `core.RequestDurationBuckets` directly. Nothing else found.
- **t006, tests.** On the fake frontend, now inexact only across a cut as Loki is: pages over two streams in a two-hour and a four-day window, a sparse instance over days read in a few parts with no listing, no re-read past the suspect page's far line, a full page inside one bucket as one request, the inclusive `endTime` on a cut, the query's own selector on every re-read, a failed re-read failing the read, a caller leaving, a busy text search reading each raw line about once, and 150 random shapes against the truth within a few times log2 of their buckets in reads. On the pinned Loki (`BEX_TEST_LOKI_URL`): w21/063's chains and a busy text search with a dense and a sparse instance across a cut. Trusting a full page fails the two-stream pages, the sparse and far-line tests, the raw-page search, the random shapes, w21/063's real-Loki chains and the real-Loki search (20 of 36 matches); trusting a full part fails the two-stream pages and the random shapes. The page histogram's labels and results, and the alert's threshold, floor and route, have their own tests.

### 2026-11-20 — unblock: the gate cleared by itself (loopx w5)

- **Affected:** w5/m503 t007. **Source:** verified evidence; no user decision was needed.
- **Finding:** production's newest pin `11b1e8f85f06` (`0779189fc`) carries `31108e539` (`git merge-base --is-ancestor`), and the date gate 2026-11-20T18:00Z has passed. The first pin carrying it was `c81d2d447bd8` (`aefb144dc`, committed 18:04Z on 2026-11-19). bex-api's two pods on that pin's image were ready at 18:09:30Z and 18:09:43Z, about 3 h after the fix's commit, and the last pod on the older image was gone by 18:10Z (kube-state-metrics `kube_pod_container_info` and `kube_pod_status_ready_time`).
- **Next:** t007 reads the 24 h from 2026-11-19T18:09:43Z to 2026-11-20T18:10Z.

### 2026-11-20 — t007: 24 h of production after the fix (loopx w5, read-only)

**How it was read.** Each bex-api pod's own counters were taken at their highest over the window and summed. No container restarted. These counts were checked against the edge's access log in Loki: 2,580 lines against 2,579 counted. The one extra line is a 401 that the auth layer answered before the route. Two sets of pins served the day:

- **A, 18:10Z to 04:26Z (10.3 h):** pins `c81d2d447bd8`, `c624a4f217ff`, `1c63b6fe0a34`, `def3eb4883de` and `98e053468f52`. They carry `31108e539` only, so a read with no `startTime` still covers seven days and a text search is still matched in bex-api.
- **B, 04:26Z to 18:10Z (13.7 h):** pins `3e53abbe3bc3`, `12716d1d7097` and `11b1e8f85f06`. They also carry `e0dc599d3` (w5/197: the default window is an hour) and `c3d4c075e` (w5/m520: Loki prefilters a text search). bex-api's first pod on `3e53abbe3bc3` was ready at 04:26:01Z.

| REST `GET /v1/logs` | A | B | 24 h |
| --- | --- | --- | --- |
| reads | 1,793 | 786 | 2,579 |
| over 5 s | 2 (0.11%) | 0 | 2 (0.08%) |
| over 10, 15, 20 or 25 s | 0 | 0 | 0 |
| p95 / p99 at the edge | 0.19 s / 1.52 s | 0.16 s / 0.32 s | 0.18 s / 1.21 s |
| slowest read | 5.41 s | 2.32 s | 5.41 s |
| answers | 1,791 × 200, one 400, one 404 | 783 × 200, two 400, one 404 | no 5xx |

**DoD 2 holds.** The share of reads over 5 s is 0.08%, against the 5% that would put the p95 at 5 s. The p95 is 0.21 s interpolated from the buckets. No read reached the 10 s bucket, none reached the 28 s REST deadline, and none answered 5xx. The week before had 2,262 reads of 9,096 between 10 and 30 s.

**By shape** (`bex_api_logs_page_duration_seconds`: every logs page on REST, GraphQL, MCP and project views, 1,848 pages in A and 1,137 in B; no page `partial`, 4 `error`):

- **The only shape over 5 s** was `1h, build, text, backward` in A: 25 pages, 2 of them over 5 s (5.27 s and 5.41 s at the edge, at 20:28Z on 2026-11-19, inside a burst of about 880 CLI reads), p95 about 6.9 s interpolated. This is cause 3 of the Findings: a text search read every raw line into bex-api. Its follow-up is w5/m520, whose prefilter `c3d4c075e` ran in B. In B the 43 text pages over 1 h and 1 d windows had p95 0.23 s or less for every shape. m520 takes its own 24 h reading.
- **Default window.** In A, `7d, default, none` had 9 pages at p95 3.9 s and `7d, default, text` 13 pages at p95 3.4 s. Both were under 5 s, and they were the slowest shapes after the one above. In B the default is an hour (w5/197): `1h, default, none, backward` had 94 pages at p95 0.10 s.
- **`path` or `host` (`line`) filter:** `7d, request, line` had 3 pages at p95 0.09 s, and `1h, request, line` 26 pages at p95 0.23 s.
- **The busiest shapes:** `1h, build, none, forward` had 1,264 pages at p95 0.13 s, and `1h, app, none, forward` 1,042 pages at p95 0.16 s.

**Who read.** 2,548 of the 2,580 reads came from Render CLI 2.36.0 on Ubuntu, in bursts of 340 to 890 an hour (20Z and 22Z on 2026-11-19, 04Z and 09Z on 2026-11-20). Browsers sent 3 and curl 16. The week's main caller before the fix sent none: that was one Render CLI 2.27.0 client on macOS, with 9,130 of 11,827 reads. So the reading rests on these 2,580 reads, not on that client's loop.

**`BexApiLogsReadSlow` could never fire.** The rule was loaded (`/etc/rules/alerting_rules.yml`, health `ok`), it stayed inactive all day, and it left no `ALERTS` series. But Prometheus 3.13.5 stores a scraped bucket `le="5"` as `le="5.0"`. The rule's `le="5"` matcher read nothing, so the rule had no value to compare. Panel 41's "reads over 5s" series read nothing for the same reason. promtool's unit test passed because its input series kept `le="5"`. Fixed here:

- The rule and the panel now match `le=~"5|5\\.0"`, as `BexOperatorAppReconcileSlowPasses` already does.
- `logs_read_alerts_test.yml` now feeds `le="5.0"`, and the old rule fails its warning case.
- `scripts/gitops-validate.sh` now refuses an integer `le` matcher in any rule or panel under `deploy/gitops`.

The corrected expression was run over the day against production Prometheus. It peaked at 0.98% at 20:58Z on 2026-11-19, during the first burst, under the 5% threshold. So the alert would rightly have stayed quiet. DoD 3 still holds: `TestMCPListLogsIsRESTsBody` passes on this tree.
