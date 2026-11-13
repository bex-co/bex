# w10 · m374 — A service's Metrics and Logs count every request and show only what its type can have

**Worker:** worker10 **Goal:** Response Times includes every request the request log records, wakes included; a worker, cron job or private service is never offered request views it can never fill; the Network filters show their values in full; and a datastore's Logs tab keeps an ended custom range closed to the live tail, as service Logs do. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Response Times counts every request, a Free service's cold-start wake included (absorbs w4/605) | 50m | — |
| [t002](t002.md) | A worker, cron job or private service gets no request metrics or request logs (absorbs w4/613) | 45m | — |
| [t003](t003.md) | The Network filters show their selected values in full (absorbs w4/614) | 15m | — |
| [t004](t004.md) | A datastore's Logs tab keeps an ended custom range closed to the live tail (absorbs w4/625) | 30m | — |
| [t005](t005.md) | Render parity | 20m | t001, t002, t003, t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 40m | t005, t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

Total: **8 tasks, about 3h45m**.

## Definition of done

On production, after the deploy that carries this milestone, as the QA user in `bex-canary`:

1. An owned Free web service with Idle Timeout 60 s is left to hibernate and requested once, then twice in a row (`curl -w '%{time_total}'`). Unfiltered `GET /v1/metrics/http-latency?quantile=0.99` and the dashboard Response Times p99 show each wake's minute within ~1 s of the measured time, matching the `&host=` read. A minute with a single request after a Suspend → Resume has a point. The path count, host filter, `aggregateBy=host` and a static site's Response Times are unchanged.
2. On a Starter background worker, Metrics shows Memory, CPU, Total Instances and Outbound Bandwidth and no request card or control; Logs list All logs and Application logs with Level and Instance filters; `…/logs?type=request` shows application lines. The same holds on a cron job and a private service; a web service's pages are unchanged.
3. On a web service's Metrics page at 1600 px and 1024 px, in en and zh, every Network filter reads its full value.
4. On a Key Value and a Postgres Logs tab, a custom range 1 hour ago → 59 minutes ago leaves Live off and disabled with the reason and sends no subscribe request; a range ending an hour from now tails.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w10` (2026-11-12/13), filed by `/pmx them all to w10` on 2026-11-13, re-checked at filing against `origin/main` `3c6f11dde`, item 3. Absorbs [w4/605](../../w4/done/605.md) (major), [w4/613](../../w4/done/613.md), [w4/614](../../w4/done/614.md) and [w4/625](../../w4/done/625.md) (`/qa-find-bugs w4` rounds 189-251, 2026-11-12/13).
- **Render:** [service-metrics](https://render.com/docs/service-metrics): HTTP request volume and response latency are "Web services only"; outbound bandwidth is "All service types". [logging](https://render.com/docs/logging): request logs are generated for requests to web services from the public internet. Render's handling of a spin-up wait in its response-time chart was not captured (w4/559's unverified line, carried over); t001 rests on bex's own request log, the same data the count already uses.
- **Parity scope, stated plainly:** t002 is Render parity; t001 is metrics correctness (a Free user's slowest request is hidden); t003 and t004 are bex's own UI contract (Render n/a).
- **Goal linkage:** Render parity first (ADR008); basic observability (`.pm/GOAL.md` item 2).
- **Expected outcome:** a Free user sees the wake they waited for in Response Times, and no service type shows an empty view that reads as "nothing happened".
- **Why now:** w4/605 is a shipped fix's DoD failing live; w4/625 is the unfinished half of w21/016; w4's queue holds 26 open notes while w10 has no actionable work.
- **Render parity:** included (metrics API and dashboard change).

## Findings

None yet.
