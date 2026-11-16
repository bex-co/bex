# w10 · m374 — A service's Metrics and Logs count every request and show only what its type can have

**Worker:** worker10 **Goal:** Response Times includes every request the request log records, wakes included; a worker, cron job or private service is never offered request views it can never fill; the Network filters show their values in full; and a datastore's Logs tab keeps an ended custom range closed to the live tail, as service Logs do. **Status:** blocked — release gate (t001–t007 done; DoD 1, 2 and 4 replayed live on pin `284b83e1f540` 2026-11-16 and hold; DoD 3 failed on a selected long Host, fixed in the follow-up commit that shipped with this park. t008 rechecks the Host trigger once a production pin carries it; no user action)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Response Times counts every request, a Free service's cold-start wake included (absorbs w4/605) — **DONE** | 50m | — |
| [t002](done/t002.md) | A worker, cron job or private service gets no request metrics or request logs (absorbs w4/613) — **DONE** | 45m | — |
| [t003](done/t003.md) | The Network filters show their selected values in full (absorbs w4/614) — **DONE** | 15m | — |
| [t004](done/t004.md) | A datastore's Logs tab keeps an ended custom range closed to the live tail (absorbs w4/625) — **DONE** | 30m | — |
| [t005](done/t005.md) | Render parity — **DONE** | 20m | t001, t002, t003, t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 40m | t005, t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

Total: **8 tasks, about 3h45m**.

## Definition of done

On production, after the deploy that carries this milestone, as the QA user in `bex-canary`:

1. An owned Free web service with Idle Timeout 60 s is left to hibernate and requested once, then twice in a row (`curl -w '%{time_total}'`). Unfiltered `GET /v1/metrics/http-latency?quantile=0.99` and the dashboard Response Times p99 show each wake's minute within ~1 s of the measured time, matching the `&host=` read. A minute with a single request after a Suspend → Resume has a point. The path count, host filter, `aggregateBy=host` and a static site's Response Times are unchanged.
2. On a Starter background worker, Metrics shows Memory, CPU, Total Instances and Outbound Bandwidth and no request card or control; Logs list All logs and Application logs with Level and Instance filters; `…/logs?type=request` shows application lines. The same holds on a cron job and a private service; a web service's pages are unchanged.
3. On a web service's Metrics page at 1600 px and 1024 px, in en and zh, every Network filter reads its full value.
4. On a Key Value and a Postgres Logs tab, a custom range 1 hour ago → 59 minutes ago leaves Live off and disabled with the reason and sends no subscribe request; a range ending an hour from now tails.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w10` (2026-11-12/13), filed by `/pmx them all to w10` on 2026-11-13, re-checked at filing against `origin/main` `3c6f11dde`, item 3. Absorbs [w4/605](../../../w4/done/605.md) (major), [w4/613](../../../w4/done/613.md), [w4/614](../../../w4/done/614.md) and [w4/625](../../../w4/done/625.md) (`/qa-find-bugs w4` rounds 189-251, 2026-11-12/13).
- **Render:** [service-metrics](https://render.com/docs/service-metrics): HTTP request volume and response latency are "Web services only"; outbound bandwidth is "All service types". [logging](https://render.com/docs/logging): request logs are generated for requests to web services from the public internet. Render's handling of a spin-up wait in its response-time chart was not captured (w4/559's unverified line, carried over); t001 rests on bex's own request log, the same data the count already uses.
- **Parity scope, stated plainly:** t002 is Render parity; t001 is metrics correctness (a Free user's slowest request is hidden); t003 and t004 are bex's own UI contract (Render n/a).
- **Goal linkage:** Render parity first (ADR008); basic observability (`.pm/GOAL.md` item 2).
- **Expected outcome:** a Free user sees the wake they waited for in Response Times, and no service type shows an empty view that reads as "nothing happened".
- **Why now:** w4/605 is a shipped fix's DoD failing live; w4/625 is the unfinished half of w21/016; w4's queue holds 26 open notes while w10 has no actionable work.
- **Render parity:** included (metrics API and dashboard change).

## Findings

**2026-11-13 (`/loopx w10`), implementation.**

- **t001:** `requestMetric` (`lego/backend/internal/metrics/service.go`) sends unfiltered `http_latency` to the request log like the count, with the Prometheus histogram as the fallback on a Loki error. REST, GraphQL and MCP share that read. Tests: the routing table (`TestUnfilteredRequestCountsComeFromTheAccessLog`, latency now access-log), a latency fallback subtest, and a real-Loki subtest that an unfiltered latency read has a point for every populated step.
- **t002:** `NetworkMetricsCard` takes `servesRequests` (`publiclyRoutable`): a worker, cron job or private service gets Outbound Bandwidth only, with the request and latency reads and Host discovery skipped (the shared STATUS_CODE/INSTANCE discovery still runs; the Application card reads INSTANCE from it). The Logs tab passes `servesRequests` to `LogViewer`, which drops the request type and request-line filters from every view, URL or typed (`withoutRequestFilters`), and the filter bar skips method/status discovery. The metrics pending skeleton draws one section and no controls for those types.
- **t003:** the four Network filter triggers use `min-w-*` instead of fixed widths; Host is capped at `max-w-72` with its full name in `title`.
- **t004:** `useCustomWindow` (shared with service Logs) gives the datastore Logs tab w21/016's rule: an ended custom range leaves Live off and disabled with `logs.liveWindowClosed`, and a future-ending range tails, bounded by its end.
- **t005 Render parity:** metrics behave alike on REST, GraphQL and MCP (one service read). The dashboard follows Render's documented split (request metrics and request logs for web services only). Left as a separate question, as w4/613 said: a static site keeps its request metrics (w4/m113), while Render lists static sites for bandwidth only.
- **t006 Simplify:** three reviews; applied the shared request-filter keys, `isCustomRange`, the in-viewer sanitization (a route-only cleanup could be bypassed by typed `method:` operators), the type-aware skeleton and one prop name. Skipped: with "All" percentiles each quantile falls back to Prometheus on its own if Loki is flaky.
- **t007 Test coverage:** network card (request views off; trigger widths), log filter bar (no request type or request filters; stale-link sanitization), datastore viewer (ended and future-ending custom ranges). `yarn lint` and `yarn test`, and the backend suite with the test database, pass.

## Unblock record (2026-11-16, `/loopx w10` worker; verified evidence)

- **Affected:** w10/m374 t008 (t001–t007 stay done).
- **Source:** verified evidence, no user decision needed.
- **Release gate — clear.** Production runs pin `284b83e1f540` (pin commit `02026e53f`, 2026-11-16 21:29Z; `deploy.yml` on `284b83e1f` succeeded 21:11Z). The pin covers the `bex`, `dashboard` and API images. `git merge-base --is-ancestor eb4890f6a 284b83e1f540` holds, and `eb4890f6a` is this milestone's last code commit.
- **Remaining gate:** none at the time of writing. t008 returned to `todo` to replay DoD 1–4 on production as the QA user in `bex-canary`. The replay below parked it again.

## Live replay — pin `284b83e1f540` (2026-11-16 22:36–23:02Z, QA user, `bex-canary`)

Fixtures, all `qa-20261116-m374-*` and deleted afterwards (DELETE 204, then GET 404): a Free `traefik/whoami:v1.10.1` web service with `idleTTLSeconds: 60`, Starter image background worker, cron job and private service, a static site from `git-fixtures/basic`, and a Free Postgres and Key Value.

1. **Response Times counts every wake — holds.** Unfiltered `GET /v1/metrics/http-latency?quantile=0.99&resolutionSeconds=60` and the `&host=` read return the same series: `22:38=0.061 22:41=23.581 22:43=22.995 22:45=12.566 22:46=0.003 22:49=12.303`.
   - The first wake measured 24.10 s (`22:41`), the post-resume wake 13.10 s (`22:45`) and a later wake 12.83 s (`22:49`). Each is within ~0.5 s of the measured time; the client time includes TLS.
   - The second wake (13.08 s) shares minute `22:43` with the post-Suspend → Resume first request, so the p99 there is the slower of the two. `&path=/wake2-m374` reads `22:43=12.56`.
   - A single request in its own minute after the resume has a point (`22:45`, and the warm `22:46=0.003`).
   - The count is unchanged: `http-requests` has `22:43=2` and 1 elsewhere, and `aggregateBy=host` gives the same values under the host label. The `/p1` path count is `22:38=1`.
   - The dashboard reads the same: its own GraphQL `HTTP_LATENCY` query at p90 / 12 h / 300 s returned `22:40=0.061 22:45=23.437 22:50=11.073`, and after choosing Percentile p99 it returned `22:40=0.06 22:45=23.57 22:50=12.18`. The same GraphQL query at 60 s / p99 matches REST point for point.
   - The static site's Response Times and count answer as before: `22:57=0.614` (unfiltered and `&host=`), count `22:57=10`.
2. **Request views only where a type can have them — holds.**
   - Worker, cron job and private service: Metrics shows Memory, CPU, Total Instances and Outbound Bandwidth. It shows no Total Requests or Response Times card and no Status Code, Host, Group by or Percentile control, and sends no `HTTP_REQUESTS`/`HTTP_LATENCY` read.
   - Their Logs list "All logs" and "Application logs". Filters holds Level and Instance only. `…/logs?type=request` opens on "Application logs" and shows the app's own lines (`m374-worker-line`, `m374-cron-line`, whoami's start-up line).
   - The web service is unchanged: all four controls and both request reads on Metrics, and "Request logs" plus Method, Status code, Request path and Request host on Logs.
3. **Network filters read their full value — fails for a selected long Host.** 64 option selections were measured: every option of the four triggers, at 1600 and 1024 px, in en and zh. 60 have `value.clientWidth >= scrollWidth`. The four failures are all Host = `qa-20261116-m374-free.onbex.co` (value 200/222 px, or 202/222 in zh).
   - The trigger has the full host in `title`, but the value is cut mid-name ("…free.onbe") with no ellipsis. The value is `[data-slot=select-value]`, a `display:flex` box with `text-overflow: clip`. A flex box's text never takes an ellipsis, so t003's `max-w-72` cap clips it.
   - w4/614's DoD allows a host "in full or with an ellipsis and a title", so this fails.
   - **Fix (shipped with this park):** `network-metrics-card.tsx` renders the selected host in its own `truncate` span inside `SelectValue`. Live, the same DOM change computes `text-overflow: ellipsis` and draws "qa-20261116-m374-free.onb…".
   - The regression test `ellipsizes a selected host and names it in full in the trigger's title` fails without the fix. `yarn lint` (one existing warning in `use-live-logs.ts`) and `yarn test` (734 files, 8665 tests) pass.
4. **An ended datastore range stays closed to the live tail — holds.** On the Key Value and the Postgres Logs tab, a custom range of 1 h ago → 59 min ago leaves Live unchecked and disabled. `aria-describedby` points to "Live tail is off — this custom range has already ended, so no new lines can arrive in it.", and a forced click sends no `/v1/logs/subscribe`. A range ending an hour from now enables Live, and one click opens `/v1/logs/subscribe?resource=<id>&startTime=…`.

**Verdict:** DoD 1, 2 and 4 hold on pin `284b83e1f540`. DoD 3 holds except for a selected long Host, which is fixed in code.

**Release condition (clears by itself, no user action):** a production pin that carries the Host fix commit. Then t008 rechecks only DoD 3's Host case: select a host longer than the trigger at 1600 and 1024 px, in en and zh, and see "…" and the full host in `title`. If that holds, the milestone closes.
