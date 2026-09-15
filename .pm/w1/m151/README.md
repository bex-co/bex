# w1 · m151 — A free web service under steady traffic still hibernates every 15 minutes

**Worker:** worker1 **Goal:** a free web service sleeps only after its idle window passes with **no inbound traffic**, as Render documents. Its idle clock advances on every request the service actually serves (and on WebSocket activity), not only when it first runs or is woken. A service that is being used never goes through a hibernate→wake cycle. **Status:** in progress — t005 and t006 done; t001–t004 implemented with tests green, and their live probes (the three DoD bullets, the WebSocket probe) wait for the deploy; then t007 closeout.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Served requests advance a free web service's idle clock (a Prometheus activity reader, checked before any hibernate) | 60m | — |
| t002 | WebSocket activity keeps a free web service awake too | 40m | t001 |
| t003 | Blast radius and adjacent states: every path that stamps, reads or bypasses the idle clock | 40m | t001 |
| t004 | Render parity | 20m | t002, t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 45m | t004 |
| t007 | Closeout | 10m | t006 |

## Definition of done

Each bullet can be repeated on a throwaway free web service (`bex-co/bex` `examples/hello-go` or `examples/stack-demo`, docker, `idleTTLSeconds: 0`) that is live and returning `200`. Watch it through `GET /v1/services/<srv>/events` and `curl`. Only states observed at filing time are listed:

- **Steady traffic keeps it awake.** From the service's first Running (or its last wake), `curl` its URL every 15 s for 20 minutes. No `service_hibernated` event appears, and every response is the app's own `200`. At filing time `service_hibernated` fired at **13:41:17Z**, exactly 15:00 after the 13:26:17Z wake. The request log showed a `200` at 13:41:02, and the probe's 13:41:17 request got a `503` from the activator.
- **An idle service still sleeps, timed from its last request.** Stop sending requests. `service_hibernated` then appears no earlier than 15 minutes after the **last** request, plus reconcile slack. At filing time the first cycle hibernated at 13:25:47Z, 15:00 after the first deploy went live (13:10:47Z) but only 2m10s after the last request (13:23:37Z).
- **Wake still works.** The first request to a hibernated service wakes it, and it serves the app's real response shortly after. At filing time that passed: 13:26:04 `503 {"error":"service hibernated","retryAfter":5}`, then 13:26:21 `200 db ok: SELECT 1 = 1`, with `service_woken` at 13:26:17Z. This milestone must keep it.

## Evidence (probes run 2026-09-14, production, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`)

Fixtures, all created and deleted inside the run:

- `qa-20260914-stack`: free web service `srv-dajv38a6m8ac739r5tjg`, `examples/stack-demo`, docker, `idleTTLSeconds: 0`.
- `qa-20260914-db`: free Postgres `dpg-dajv2r26m8ac739r5tg0`, later renamed `qa-20260914-db-renamed`.
- `DATABASE_URL` on the service was set to the database's internal connection string.
- Both were deleted at 13:46:04Z (`DELETE` → `204`, then `GET` → `404`). The URL returned `404` at 13:46:28Z.

1. **Timeline** (events from `GET /v1/services/srv-dajv38a6m8ac739r5tjg/events`; requests from `curl` outside the cluster):

   ```text
   13:10:47Z deploy_ended  (first deploy live)
   13:12:53  curl → 200 "db ok"  … 200s until 13:17:58, then app 503s (database suspended on purpose) every 5 s until 13:23:37
   13:25:47Z service_hibernated         ← 15:00 after 13:10:47, 2m10s after the last request
   13:26:04  curl → 503 {"error":"service hibernated","retryAfter":5}
   13:26:17Z service_woken
   13:26:21  curl → 200 "db ok"
   13:27:56 … 13:41:01  curl every ~15 s → 200 (54 samples)
   13:41:17Z service_hibernated         ← 15:00 after the 13:26:17 wake, under steady traffic
   13:41:17  curl → 503
   13:41:32 … 13:45:00  curl → 200 (12 samples); 13:41:47Z service_woken
   ```

2. **The platform saw the traffic it ignored.**
   - `GET /v1/logs?resource=srv-dajv38a6m8ac739r5tjg&type=request&limit=100&direction=backward&startTime=2026-09-14T13:39:00Z&endTime=2026-09-14T13:44:00Z` returned a `200` record every ~15 s: 13:39:12, 13:39:27, …, 13:40:46, 13:41:02, then 13:41:33, 13:41:49, …
   - Every record carried Traefik `ServiceName` `tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20260914-stack-3000@kubernetes`.
   - The activator-served request at 13:41:17 is not in the service's request log.
   - `GET /v1/metrics/http-requests?resource=srv-dajv38a6m8ac739r5tjg&startTime=2026-09-14T13:26:00Z&endTime=2026-09-14T13:32:00Z&resolutionSeconds=60` returned non-zero request rates of 0.04–0.13 req/s for 13:27–13:32.
3. **Render's contract** (`render.com/docs/free`, fetched 2026-09-14): "Render spins down a Free web service that goes 15 minutes without receiving any inbound traffic. This includes both HTTP requests and WebSocket messages from existing connections."
4. **The dashboard's promise** (Settings idle timeout hint, `dashboard/src/features/services/locales/en.ts:1092`): "Free services sleep after this idle window, then wake on the next request."

No screenshots were taken; the transcripts above are the evidence.

## Root cause

- **The decision reads only a timestamp.** `shouldAutoHibernate` (`lego/operator/internal/controller/app_controller.go:1942-1951`) returns `time.Since(lastActive) >= autoSleepWindow(app)`. No traffic is consulted.
- **Nothing that serves an awake service writes that timestamp.** An exhaustive grep for `annotLastActive` / `app.bex.co/last-active` across `lego` (non-test) finds 13 references with exactly **two writers**:
  - `runningRequeue` stamps it only when it is **absent** (`app_controller.go:2405-2416`), i.e. on first Running.
  - The activator's `wakeApp` stamps it (`lego/operator/cmd/activator/main.go:169-196`), but the activator is only in the request path while the Ingress routes to it, i.e. while the service is hibernated or has no ready pod (`ingressBackend`, `app_controller.go:2340-2347`).
- **The comment is wrong.** The annotation's own doc ("updated by the activator on each inbound request", `app_controller.go:220-224`) describes only the asleep case; an awake service's traffic goes straight to its own Service.
- **No other signal exists.** No Traefik plugin records activity (`deploy/gitops/charts/traefik-plugins/` holds only `websocketegress`). The only per-App request signal, `traefik_service_requests_total`, feeds bex-api metrics and Grafana, never the operator.
- **Why it now bites every free service.**
  - `w6/m116` (`0ca167a87`) made `idleTTLSeconds: 0`, the create default, mean a 15-minute window instead of "never". Every created-default free web service became eligible.
  - `TestShouldAutoHibernate` (`idle_test.go:162-255`) varies only the stamp's age, so no test ever asked whether requests after the stamp matter.
  - `w4/done/068` measured "last request 14:06:47 → hibernated 14:21:47". That last request was the wake request itself, which is the one request that does stamp.

## Blast radius

- **Every free web service**, platform-wide, that is live and receiving traffic: `autoSleepEligible` (`app_controller.go:1935-1938`) covers `web_service` on the free plan, not suspended. Such a service cycles hibernate→wake every `autoSleepWindow` (15 min by default, or the explicit `idleTTLSeconds`).
  - Each cycle scales the Deployment to 0, routes to the activator, and answers at least one real request with a `503` (JSON for API clients, the wake page for browsers).
  - Each cycle then pays a cold start and writes a `service_hibernated`/`service_woken` pair.
  - Paid plans, private services, workers, cron jobs and static sites are unaffected (`autoSleepWindow` returns 0 or they are not eligible).
- **This workspace has no other free services** (all five pre-existing services are starter or standard), so no second service was observed. Other workspaces were not read.
- **The fix seam already exists.**
  - The operator gets `BEX_PROM_URL` (`lego/operator/config/manager/manager.yaml:215`), today wired only into the database reconciler (`cmd/manager/main.go:509-510`).
  - The operator has `promInstantQuery` (`internal/controller/autoscale.go:136`).
  - The Traefik service label is `<namespace>-<app>-<port>@kubernetes` (bex-api's `traefikServiceLabel`, `lego/backend/internal/metrics/source.go:373-375`). The operator cannot import the backend, so the helper is duplicated or moved to `types`.

## Adjacent classes

- **Activator-served requests** must keep stamping on wake, as today.
- **Routing hold** (`w6/m94`): the grace window must not start a sleep while a pod is not yet ready.
- **Maintenance mode** keeps its precedence over auto-sleep (`TestMaintenanceModePrecedesAutoHibernate`).
- **Explicit `idleTTLSeconds`** (300…7200): the same activity rule applies with that window.
- **Paid, private, worker, cron and static** stay never-sleep, unchanged.
- **Prometheus unavailable or erroring** must fail **awake**: skip the hibernate, log, and requeue. A missing metrics backend must never turn into a sleep for a service that may be busy.
- **A service returning 5xx** is still receiving traffic. Render counts inbound traffic, not successful responses, so requests count regardless of status.

## Unverified (reasoned, not probed this run)

- **WebSocket traffic** was not exercised. Whether the `websocketegress` plugin's per-App counter (`:9101`) is scraped by Prometheus was not checked (t002).
- **Cold-start cost.** The cold start for a heavier app, and what a browser sees mid-cycle, were not probed. The wake page's content negotiation is recorded as deliberate in `w4/done/068`.
- **Wake ordering.** The ordering between the Deployment scale-down and the 13:41:32 `200` (before `service_woken` at 13:41:47Z) was not traced.
- **Prometheus timing.** The scrape interval and counter-reset behavior across a hibernate (pods gone, series stale) are to be checked in t001.

## Implementation (2026-09-14)

**Served traffic advances the idle clock (t001).** `lego/operator/internal/controller/activity.go`:

- `AppActivityReader` returns the latest time after `last-active` at which the App served traffic. `NewPrometheusAppActivityReader` runs one query, `max_over_time(timestamp((sum(increase(<requests>[1m])) > 0) or (sum(increase(<ws frames>[1m])) > 0))[<lookback>:15s])`: the latest 15 s step at which either counter rose over the preceding minute. The series are `traefik_service_requests_total{service="<namespace>-<app>-<port>@kubernetes"}` (every status — Render counts inbound traffic, not successes) and `bex_websocket_egress_bytes_total{app_id=…}`. The lookback is the time since the stamp, capped at the window plus one step: older traffic cannot keep the App awake, and an uncapped days-old stamp would ask for a subquery that times out. The answer trails a request by at most a minute plus a scrape, which only ever delays a sleep. `increase()` is reset-safe, and a series that went stale when the service was parked yields nothing.
- `desiredReplicas` hibernates only when `shouldAutoHibernate(app)` (the stamp is past the window, the cheap gate, unchanged) **and** `!recentlyActive(ctx, app)`. `recentlyActive` advances `app.bex.co/last-active` to the observed traffic and reports whether that traffic is still inside the window, so the next check is timed from the last request (`idleRequeueAfter`).
- **Fail awake:** a reader error, or a failed stamp patch, keeps the service awake and leaves the stamp alone; `runningRequeue` re-asks after one minute instead of the 5 s floor. After a failed read the reader answers "unavailable" for 30 s without querying, so a hung Prometheus cannot hold a reconcile worker for its full request timeout on every awake free service in turn. The stamp patch carries its resourceVersion, so a stale cached copy conflicts instead of moving a newer activator wake stamp backwards. **No reader** (no `BEX_PROM_URL`, local clusters) keeps the stamp-only rule, so the existing idle, wake and maintenance tests run unmodified. A **hibernated** App is not read: its route is the activator, and its own Service serves nothing.
- Wiring: `cmd/manager/main.go` `appActivityReader()` from `BEX_PROM_URL`, the Prometheus the database disk autoscaler already reads (`config/manager/manager.yaml` sets it in production). The `last-active` annotation's doc now names its three writers.

**WebSocket (t002).** Prometheus already scrapes the plugin's `:9101` counter (`deploy/gitops/base/prometheus.yaml` job `traefik-websocket-meter`, which keeps `bex_websocket_egress_bytes_total`), so no scrape change was needed. That counter measures **server→client** frames only: a connection on which only the client sends messages does not keep a service awake. Recorded as a divergence in ADR018 and filed as `w1/102`. The live 20-minute WebSocket probe waits for the deploy.

**States (t003).**

| State | Behavior | Test |
| --- | --- | --- |
| Awake, stamp past the window, traffic inside it | Stays awake; stamp advances to the traffic | `TestIdleDecisionConsultsServedTraffic` (row 1), `TestSteadyTrafficKeepsAFreeServiceAwakeThenItSleepsWhenQuiet` |
| Awake, traffic after the stamp but older than the window | Sleeps; stamp still advances | `TestIdleDecisionConsultsServedTraffic` (row 2) |
| Awake, no traffic since the stamp | Sleeps, a window after the last request | `TestIdleDecisionConsultsServedTraffic` (row 3), the quiet half of `TestSteadyTraffic…` |
| Metrics unreadable | Awake, stamp untouched, re-asked after 1 min | `TestIdleDecisionConsultsServedTraffic` (row 4) |
| Stamp inside the window | Traffic not read (unchanged) | `TestIdleDecisionConsultsServedTraffic` (row 5), `TestShouldAutoHibernate` |
| Hibernated | Not read; wakes through the activator's stamp, as before | `TestIdleDecisionConsultsServedTraffic` (row 6), `TestWakeRestoresAppServiceAsIngressBackend` |
| Waking, routing hold (`w6/m94`) | The wake stamp is fresh, so traffic is not read; the hold is unchanged | `TestWakeRestoresAppServiceAsIngressBackend` (unmodified) |
| Maintenance mode | Precedence unchanged | `TestMaintenance*` (unmodified) |
| Manually suspended; paid, private, worker, cron, static | Never eligible, never read | `TestShouldAutoHibernate` rows (unmodified) |
| No `BEX_PROM_URL` | Stamp alone decides | `TestIdleDecisionWithoutAReaderUsesTheStamp` |

- **Rollouts do not restamp.** A deploy is not inbound traffic (Render counts requests and messages); traffic served during and after a rollout is what keeps the service awake.
- **Events.** `service_hibernated`/`service_woken` now follow real idleness. The push-notification exclusion (`lego/backend/internal/notifications/push_worker.go`, `w6/m47`) is untouched.

**Parity and copy (t004).** The dashboard idle-timeout hint (en + zh) and the `IDLE_TIMEOUT_PRESETS` doc, the MCP `idleTTLSeconds` description, the CRD field doc (`lego/types/v1alpha1/app_types.go`, CRD regenerated), ADR003 § free web tier and ADR018's Suspend/Resume row now say the window counts time without traffic. The dashboard says "without inbound requests or WebSocket messages"; the CRD and MCP text are exact ("HTTP requests it served, or WebSocket frames it sent"), matching the server→client divergence above.

**Simplify (t005).** Three review passes (reuse, quality, efficiency). Applied:

- The lookback is capped at the window plus one step. A days-old stamp (a long outage, or an App that was paid for months) would have asked for a subquery past Prometheus's request timeout, erroring and keeping the service awake indefinitely.
- A 30 s breaker after a failed read. Every awake free service asks during an outage, and each hung query held one of the operator's two reconcile workers for 5 s, which would have starved other Apps' deploys.
- The two signals are one `or` query: half the round trips and the worst-case blocking.
- One `stampLastActive` helper for the first-Running stamp and the traffic advance, with an optimistic lock (see Fail awake).
- `recentlyActive` returns `!shouldAutoHibernate(app)` after stamping, so one rule decides.
- The CRD and MCP wording says "WebSocket frames it sent", not "messages", and a doc comment misplaced in `main.go` is back on `envOr`.

Declined:

- A shared Prometheus base-URL-and-client type with the disk-usage reader; it would save three lines.
- A shared Traefik service-label helper in `lego/types` for a one-line format also used by bex-api.
- A health condition for a misconfigured `BEX_PROM_URL`; failures are logged and fail awake.
- Rewording the tests' filing-time comments; they follow the repo's convention.

**Tests (t006).** `activity_test.go`: the decision table above; requeue timed from the traffic; a full reconcile that keeps the replica under traffic and scales to 0 once quiet; the reader against a fake Prometheus (one query carrying the exact service label and `app_id`, the lookback capped at the window for a three-day-old stamp, no status matcher, the latest sample wins, nothing newer than the stamp, and one failed read opening the breaker so the next read makes no request). **Shown failing without the fix:** in a scratch worktree with only the traffic read removed from `desiredReplicas` (and the slower recheck from `runningRequeue`), `TestIdleDecisionConsultsServedTraffic` (4 rows), `TestRequeueAfterTrafficIsTimedFromTheTraffic` and `TestSteadyTraffic…` failed ("auto-hibernating = true, want false"; "replicas under steady traffic = 0, want 1"). The reader tests cover a type that did not exist pre-fix.

## Dedupe

- `w6/done/m47`, `m93` and `m94` fixed the wake route and routing races, and `w6/done/m116` fixed the default window. None covers traffic while awake.
- `9917191fe` stopped **private** services sleeping because they had "no activity signal". Public web services have the same missing signal while awake.
- `w4/done/068` verified sleep timing only against a wake request (see Root cause).
- `.pm/DO_NOT_DO.md` line 34 (`w6/m120`, "a hibernated free service wakes itself with no traffic") is the opposite direction and does not apply.
- No open item covers this.
- Not fixed on `main` as of `fce26978b`: `git log -S"annotLastActive"` / `-S"shouldAutoHibernate"` since 2026-08-20 show only m47/m93/m94/m116/`9917191fe`.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-14 pass 12. Journeys 11 and 15 were combined: a free web service wired to a free Postgres through its internal URL, exercised across rename and suspend/resume, with its sleep/wake cycle watched under steady traffic.
- **Goal linkage:** `docs/ADR003-control-plane.md` § "Free web tier = sleep-when-idle (decided)" (line 80). It is also Render parity for the free tier (`render.com/docs/free`) and the idle-window contract `w6/m116` settled.
- **Expected outcome:** a free web service that people are using stays up. It sleeps only after 15 quiet minutes, and hibernate/wake events mean the service was actually idle.
- **Why now:** since `w6/m116`, every created-default free web service is on this clock. A used free service drops a request and cold-starts every 15 minutes, which reads to its users as the platform being unreliable.
- **Render parity:** included. There are no REST/GraphQL/MCP shape changes, but the dashboard's idle-timeout hint, the MCP `idleTTLSeconds` description, and the events feed all describe this behavior. t004 aligns them with Render's "without receiving any inbound traffic".
