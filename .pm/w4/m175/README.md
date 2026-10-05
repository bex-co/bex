# w4 · m175 — Total Requests drops the newest bucket: 24h/7d windows read "No data" for recent traffic

**Worker:** worker4 **Goal:** `http_requests` counts every request in `[start, end]` for every resolution, including the bucket that contains `end`, so N driven requests sum to N on every window and surface (the full w4/m119 guarantee, "for any query alignment"). **Status:** todo

## Tasks (in order)

| id   | title                                                               | est | depends_on   |
| ---- | ------------------------------------------------------------------- | --- | ------------ |
| t001 | Include the bucket containing `end` in Loki-backed request reads    | 45m | —            |
| t002 | Render parity                                                       | 20m | w4/m175/t001 |
| t003 | Simplify                                                            | 15m | w4/m175/t002 |
| t004 | Test coverage                                                       | 45m | w4/m175/t002 |
| t005 | Closeout                                                            | 10m | w4/m175/t004 |

## Definition of done

Live on an owned Free web service (`traefik/whoami`, `WHOAMI_PORT_NUMBER=8080`, port 8080; deleted afterwards). Send exactly 100 `GET` and 25 `POST` to `/<marker>`, wait 1 min, then read **within one bucket of the traffic** (before the next epoch-aligned boundary):

- **Every range counts the newest bucket.** GraphQL `metrics(query:{name:HTTP_REQUESTS, filters:[{field:"RESOURCE", values:[<id>]}], start:<now-span>, end:<now>, resolution:<r>})` sums to the driven count (plus any separately counted readiness probes) for each dashboard preset: `(1800,15) (3600,30) (14400,120) (43200,300) (86400,720) (172800,1440) (604800,5040) (1209600,10080)`. Today `(86400,720)` and `(604800,5040)` return **no series** (sum 0), while `end:<now+720>` returns 127.
- **Same with filters.** `&path=/<marker>` sums to exactly 125 on every preset (today 125 at 60 s only).
- **The last point covers `end`.** The final bucket's timestamp is at or after the newest request, so a chart never ends before "now". Today the last 24h Loki point is the step boundary at or before `end`.
- **Dashboard.** Metrics → Network Metrics → Total Requests on "Last 24 hours" and "Last 7 days" shows the bars and the request total, not "No data in range", while Response Times and Outbound Bandwidth already show the same traffic.
- **REST / GraphQL / MCP agree** for the same window, and m119's bullets still hold: 60 s windows exact; counts reconcile with `GET /v1/logs?type=request`.

## Source + Goal linkage

- **Source:** infinite `/qa-find-bugs` loop46, 2026-10-05 UTC, muse.env, `bex-canary`, pin `c7afefad5`. Fixture `qa-20261005-l46-met` (`srv-db1rpt8ti8qc73bltd1g`, deleted afterwards).
  - Traffic: 100 GET + 25 POST to `/l46probe` between 15:06:41 and 15:07:52, all 200.
  - REST `GET /v1/metrics/http-requests?resource=<id>&resolutionSeconds=60` over 15 min: 33 + 94 = 127 (125 driven + 2 readiness probes); `&path=/l46probe` → 31 + 94 = 125.
  - GraphQL at 15:10–15:11 by preset: 30m sum 127; 1h 127; 4h 127; 12h 127 (one point); **24h: 0 series; 7d: 0 series**.
  - Same 24h read with `end` = now + 720 s: one point `15:12:00 = 127`.
  - Control `BANDWIDTH` over the same 24h/7d: 121 points, the last at `15:11:08` (= `end`), sum 54287 bytes. Prometheus evaluates at `end`; the Loki read does not.
  - Dashboard: at 15:09–15:10 "Last 12 hours" showed Total Requests "No data in range" (bucket 15:00–15:05 had nothing yet). At 15:11:51 "Last 24 hours" showed "No data in range" while Response Times and Outbound Bandwidth showed the traffic. Screenshot `.playwright-mcp/qa-metrics-24h.png` (session-local).
- **Root cause:**
  - `lego/backend/internal/metrics/service.go:996-1023`: unfiltered `http_requests` reads are served by the Loki request-log source since w4/m119 (exactness).
  - `lego/backend/internal/metrics/lokisource.go:60-66` passes `start/end/step` to Loki `query_range`, and `lokiRequestQueryFor` (`:107-110`) counts `count_over_time(...[step])`.
  - The returned buckets are epoch-aligned (`15:12:00` with `step=720`). With `end=now`, the last evaluation is the boundary at or before `now`, so requests after that boundary are in no bucket: up to one full resolution of the newest traffic (12 min on 24h, 84 min on 7d) disappears.
  - The Prometheus-backed metrics are evaluated at `end` itself, which is why Bandwidth and Latency on the same page see the traffic.
- **Fix target:** align `end` up to the next step boundary (so the evaluation at or after `end` exists), or evaluate the final partial bucket separately and merge. The trailing bucket's count must include every request in `(lastBoundary, end]`.
  - Do not double count across adjacent buckets.
  - Keep `start` handling exact so the first bucket does not pull in pre-window requests.
  - Applies to the Loki `http_latency` host/path path too (same `query_range`), and to MCP `http_request_count` (shared core).
- **Regression of:** `w4/done/m119` ("N driven requests sum to N ±1 … for any query alignment"). m119's DoD probed only `resolutionSeconds=60`, so coarse presets were never exercised. Its bullets 2–4 must still hold (re-checked in t004).
- **Governing docs:** ADR010 (observability), ADR018 metrics row.
- **Goal linkage:** ADR008 truthful hosting dashboard; Render parity of the Metrics page.
- **Expected outcome:** a fresh deploy's traffic appears on every range immediately; totals do not depend on the wall-clock minute.
- **Why now:** the 24h/7d defaults read "No data in range" for any service whose traffic is younger than one bucket. That is exactly the "did my deploy get traffic?" check after a release.
- **Render parity:** included (t002). The numbers on REST/GraphQL/MCP/UI change; the shapes do not.
- **Unverified:** whether Loki or bex-api's window builder does the epoch alignment (t001 reads Loki's `query_range` step handling at the pinned version); 2d/14d presets (reasoned the same). (An earlier note that `HTTP_LATENCY` read `0` at 24h was a probe artifact: the probe rounded sub-second seconds to integers. Retracted 2026-10-05.)
- **Severity:** major.
