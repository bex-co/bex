# w5 · m370 — A spec change takes effect in seconds: the operator's App work queue is measured, alerted on and kept short

**Worker:** worker5 **Goal:** the operator's App controller queue is visible on obs.bex.co, an alert fires when it backs up, and the cause of the intermittent minute-long lag is found and fixed, so a settings change such as maintenance mode reaches the cluster within seconds. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Keep the App controller's work-queue and reconcile-time metrics, with recording rules and an ADR088 panel | 40m | — |
| [t002](t002.md) | Reproduce the queue wait on the mock cluster at production-like load and find the slow passes | 1h30m | t001 |
| [t003](t003.md) | Keep the App queue short: fix the cause t002 measured | 1h30m | t002 |
| [t004](t004.md) | Alert when the App queue backs up, with a promtool test | 40m | t001 |
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
