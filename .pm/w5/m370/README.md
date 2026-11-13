# w5 · m370 — A spec change takes effect in seconds: the operator's App work queue is measured, alerted on and kept short

**Worker:** worker5 **Goal:** the operator's App controller queue is visible on obs.bex.co, an alert fires when it backs up, and the cause of the intermittent minute-long lag is found and fixed, so a settings change such as maintenance mode reaches the cluster within seconds. **Status:** in progress (t001, t004 done: the queue metrics, recording rule, alert and Builds-dashboard panels are shipped; t002 next)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Keep the App controller's work-queue and reconcile-time metrics, with recording rules and an ADR088 panel — **DONE** | 40m | — |
| [t002](t002.md) | Reproduce the queue wait on the mock cluster at production-like load and find the slow passes | 1h30m | t001 |
| [t003](t003.md) | Keep the App queue short: fix the cause t002 measured | 1h30m | t002 |
| [t004](done/t004.md) | Alert when the App queue backs up, with a promtool test — **DONE** | 40m | t001 |
| [t005](t005.md) | Simplify | 15m | t003, t004 |
| [t006](t006.md) | Test coverage | 30m | t005 |
| [t007](t007.md) | Closeout (24 h production reading and live replay) | 30m | t006 |

Total: **7 tasks, about 5h35m**.

## Definition of done

1. `workqueue_queue_duration_seconds`, `workqueue_work_duration_seconds`, `workqueue_depth` and `controller_runtime_reconcile_time_seconds` for the App controller (`name`/`controller` = `app`) are stored by Prometheus and paneled on the ADR088 dashboard at obs.bex.co.
2. On the mock cluster with about 100 Apps (builds in flight among them), the App queue's p95 wait is 5 s or less after t003 (t002 records the before).
3. On production, after the deploy: a 24 h reading of the queue wait is recorded in § Findings and appended to [w21/blocked/012](../../w21/blocked/012.md); turning maintenance mode on and then off on a Starter web service in `bex-canary` flips the public host within 15 s, in 5 of 5 tries (round 181 measured 80 s and 55 s).
4. `BexOperatorReconcileQueueSlow` and its promtool test pass in `scripts/gitops-validate.sh`.
5. Every fixture is deleted and each GET answers 404.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` round 9, item 2 (2026-11-12), filed by `/pmx 1,2,3,4` with w5 chosen. Re-checked at filing against `origin/main` `98b3587a8`.
- **Evidence:**
  - [w4/603](../../w4/603.md): enabling maintenance mode took ~80 s to repoint the host's Ingress and disabling it ~55 s, while the same two changes minutes later took 11 s and 7 s. (w4/603's own fix, the activator's wrong 503 during the lag, stays with w4.)
  - [w21/blocked/012](../../w21/blocked/012.md) addendum (round 181): an image deploy sat `queued` 3 m 06 s; "bex-api's projection to the App CR was fast while the operator's reconcile of that App lagged by a minute or more, intermittently … the operator's workqueue is the first thing to read once logs are available". The note's earlier Blueprint deploys sat `queued` for 20–35 minutes.
  - Nothing can read it today: the `bex-operator` scrape job (`deploy/gitops/base/prometheus.yaml:556-570`) keeps only `bex_(build|datastore).*`, and its comment says to add a family "when something actually consumes it". This milestone's alert and panel are those consumers.
  - Production runs `BEX_APP_RECONCILE_WORKERS=2` for every App in every workspace (`lego/operator/config/manager/manager.yaml:181`, read at `cmd/manager/main.go:409`). Builds are observed non-blocking every 5 s (`app_controller.go:1059`), and `app_controller.go` has 48 `RequeueAfter` sites; the controller is `Named("app")` with a generation-or-deletion predicate (`app_controller.go:7714-7740`).
- **Goal linkage:** GOAL item 2 (basic observability for operation); Render parity first (ADR008) in effect, since a Render setting takes effect promptly.
- **Expected outcome:** the minute-long lags become visible, alerted and fixed; w21/012 becomes diagnosable without cluster shell access.
- **Why now:** two QA workstreams keep hitting the lag with no way to see its cause, and it grows with the number of Apps.
- **Render parity: omitted.** Operator mechanism and platform observability only; no REST, GraphQL, MCP or dashboard change. The tenant-visible effect is latency, checked by the DoD's live replay.
- **Risk:** reading obs.bex.co needs the ops-workspace gate (ADR088). If the worker lacks it, the Closeout's production reading needs the user to read and paste the panel.

## Findings

### 2026-11-13 — t001 and t004 (loopx w5)

- **Kept:** the `bex-operator` scrape now keeps `workqueue_depth`, `workqueue_queue_duration_seconds_*`, `workqueue_work_duration_seconds_*`, `controller_runtime_reconcile_time_seconds_*`, `controller_runtime_active_workers` and `controller_runtime_max_concurrent_reconciles` beside `bex_(build|datastore)` (`deploy/gitops/base/prometheus.yaml`).
- **Recording rule:** `bex:operator_app_queue_wait_seconds:p95` (the `app` queue, 5 m rate).
- **Alert:** `BexOperatorReconcileQueueSlow` (p95 over 15 s for 10 minutes, warning), with `deploy/gitops/base/rules/operator_queue_alerts_test.yml`: a sustained 20–60 s wait fires, a healthy queue and a five-minute spike do not.
- **Panels:** two on the Builds dashboard (`bex-builds`, ids 11 and 12): queue wait p50/p95 with depth, and reconcile p95 with busy and maximum workers. `scripts/obs-coverage-check.sh` maps the alert to them.
- **t002 tooling:** this host had neither `kind` nor `clusterctl`, so `scripts/mock-cluster.sh` could not run. Installed the versions CI pins (kind v0.33.0 from `.github/workflows/egress-meter-live.yml`, clusterctl v1.13.2 from `.github/workflows/app-cluster.yml`) into `~/go/bin` and `~/.local/toolchains/bin`.
