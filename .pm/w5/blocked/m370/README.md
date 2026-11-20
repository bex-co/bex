# w5 · m370 — A spec change takes effect in seconds: the operator's App work queue is measured, alerted on and kept short

**Worker:** worker5 **Goal:** the operator's App controller queue is visible on obs.bex.co, an alert fires when it backs up, and the cause of the intermittent minute-long lag is found and fixed, so a settings change such as maintenance mode reaches the cluster within seconds. **Status:** blocked (t001 to t006 done and shipped; t007's handovers, toggles and fixture cleanup done 2026-11-20; its 24 h queue reading waits for 2026-11-20T18:10Z, a day after the operator first ran `500c5974b`, see § Findings)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Keep the App controller's work-queue and reconcile-time metrics, with recording rules and an ADR088 panel — **DONE** | 40m | — |
| [t002](done/t002.md) | Reproduce the queue wait at production load and find the slow passes (production's queue readings; the mock cluster cannot run here) — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | Keep the App queue short: fix the cause t002 measured — **DONE** | 1h30m | t002 |
| [t004](done/t004.md) | Alert when the App queue backs up, with a promtool test — **DONE** | 40m | t001 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t003, t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t005 |
| [t007](t007.md) | Closeout (24 h production reading and live replay) | 30m | t006 |

Total: **7 tasks, about 5h35m**.

## Definition of done

1. `workqueue_queue_duration_seconds`, `workqueue_work_duration_seconds`, `workqueue_depth` and `controller_runtime_reconcile_time_seconds` for the App controller (`name`/`controller` = `app`) are stored by Prometheus and paneled on the ADR088 dashboard at obs.bex.co.
2. On production (re-scoped 2026-11-19 from the mock cluster, see § Findings), the App queue's 24 h p95 wait is 5 s or less after t003, and an App whose spec change or deploy is in flight when the operator restarts is not queued behind the restart's fleet sweep (t002 records the before).
3. On production, after the deploy: a 24 h reading of the queue wait is recorded in § Findings and appended to [w21/012](../../../w21/done/012.md); turning maintenance mode on and then off on a Starter web service in `bex-canary` flips the public host within 15 s, in 5 of 5 tries (round 181 measured 80 s and 55 s).
4. `BexOperatorReconcileQueueSlow` and its promtool test pass in `scripts/gitops-validate.sh`.
5. Every fixture is deleted and each GET answers 404.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` round 9, item 2 (2026-11-12), filed by `/pmx 1,2,3,4` with w5 chosen. Re-checked at filing against `origin/main` `98b3587a8`.
- **Evidence:**
  - [w4/603](../../../w4/603.md): enabling maintenance mode took ~80 s to repoint the host's Ingress and disabling it ~55 s, while the same two changes minutes later took 11 s and 7 s. (w4/603's own fix, the activator's wrong 503 during the lag, stays with w4.)
  - [w21/012](../../../w21/done/012.md) addendum (round 181): an image deploy sat `queued` 3 m 06 s; "bex-api's projection to the App CR was fast while the operator's reconcile of that App lagged by a minute or more, intermittently … the operator's workqueue is the first thing to read once logs are available". The note's earlier Blueprint deploys sat `queued` for 20–35 minutes.
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

### 2026-11-13 — t002 gate (loopx w5)

- **The local cluster cannot run here as configured.** With kind and clusterctl installed, `bash scripts/mock-cluster.sh` provisioned the management cluster and the app cluster's control plane, but the CAPD tenant worker exited at boot: `Failed to create control group inotify object: Too many open files` (`docker logs bex-tenant-0-…`). The host has `fs.inotify.max_user_instances = 128` and `max_user_watches = 65536`, kind's documented limits for several nodes. A re-run reached the same state and asked for `reprovision`, which would fail the same way.
- **Raising the limit needs root:** `sudo sysctl -w fs.inotify.max_user_instances=512 fs.inotify.max_user_watches=524288` (persist it in `/etc/sysctl.d/` if wanted). Then `bash scripts/mock-cluster.sh reprovision`, `bash scripts/mock-cluster.sh operator`, and the load harness this run prepared: about 100 `traefik/whoami` App CRs in a `m370-load` namespace, reading the operator's `workqueue_*`/`controller_runtime_*` series and timing a `spec.maintenanceMode` toggle to the Ingress repoint.
- **Or measure production instead.** t001's series reach Prometheus with the next production deploy. The Builds dashboard's operator panels (obs.bex.co, ADR088 ops gate) then show the queue wait, depth, reconcile time and busy workers under the real fleet, which is the evidence t002 exists to find. Reading them needs a worker with the ops-workspace gate, or the user pasting the panels.
- **Either path unblocks t002 and t003;** t005–t007 follow.

### 2026-11-16 — production readings from [w21/m421](../../../w21/done/m421/README.md) (loopx w21; no change to this milestone's verdict or tasks)

w21/m421 read the operator's queue on production with the series t001 and t004 here shipped, and found and fixed its causes, so these numbers bear on this milestone's definition of done:

- **DoD 1 (series stored and paneled): met on production** since t001's deploy. `workqueue_*` and `controller_runtime_*` for `app` are in Prometheus and on the Builds dashboard, and `BexOperatorReconcileQueueSlow` and `BexOperatorAppReconcileSlowPasses` are loaded and inactive.
- **The cause, which t002's load reproduction was meant to find:** the manager ran at `limits.cpu: 500m`, throttled in 92% to 99% of its CFS periods for three days, and 92.7% of its CPU was the private-network port sweep (profile through the pprof endpoint t001 of w21/m421 added). Fixed by w21/m421 t001 (limit 2 CPUs) and t002 (the sweep backs off to 16 minutes and runs two rescans at once); `BEX_APP_RECONCILE_WORKERS` stays 2.
- **DoD 2 (p95 wait of 5 s or less), measured on production instead of the mock cluster:** over 55 minutes after the deploy, App queue wait p50 0.0 s, p95 0.005 s, p99 0.68 s; reconcile p50 0.09 s, p95 0.90 s; 0.13 of 2 workers busy; 0.11 CPU cores (before: p95 28 to 58 s, p50 reconcile 2.1 to 2.5 s, 1.4 to 1.5 workers busy).
- **DoD 3's maintenance toggles, 5 of 5 within 15 s: met.** Enabling flipped the public host of a Starter service in `bex-canary` in 2.8, 1.4, 1.3, 1.1 and 1.4 s, disabling in 1.7, 2.0, 1.2, 1.4 and 1.6 s (round 181 measured 80 s and 55 s); the service was deleted afterwards. The 24 h reading is not yet taken: the 55 minute window above is what exists.
- Left for whoever closes this milestone: t002's mock-cluster reproduction is no longer needed for the cause; t003, t005 and t007's remaining value is the 24 h reading and a decision whether the mock-cluster p95 check is still wanted.

### 2026-11-19 — unblocked: a worker session reads production's queue (loopx w5, `/pm unblock`)

- **Affected:** t002, t003 and DoD 2. **Source:** verified evidence plus the `/loopx` dispatch's direction (measure on production; any generated load in `bex-canary`, cleaned up). Not a new user decision.
- **The gate's second alternative holds.** Production's pin `dff52c596d13` (`a91e9aeb3`) carries t001's `5bfc00af7` (`git merge-base --is-ancestor`). A worker session reads production Prometheus without the obs.bex.co ops gate: `scripts/fetch-app-kubeconfig.sh <path outside the repo>`, then `kubectl -n monitoring port-forward svc/prometheus-server`. `workqueue_*{name="app"}` and `controller_runtime_*{controller="app"}` are stored, and `BexOperatorReconcileQueueSlow` is loaded and inactive. The first alternative (the host's inotify limits for the local kind cluster) still needs the user's sudo and is not used.
- **Scope change (DoD 2):** measured on production instead of on the mock cluster at about 100 Apps. Basis: the cause t002 exists to find is readable on production at its real fleet (28 to 29 Apps), and the local cluster cannot run here. A production load of about 70 extra services is not generated: at the Free plan's 512 Mi request it needs about 35 GiB against about 19 GiB of headroom on the serving tenant nodes today, so it would page in new nodes or hold real tenants' deploys. t002 measures the per-App cost instead, so the result at 100 Apps is computed from measured numbers, not assumed.
- **Next:** t002 records production's readings and the cause; t003 fixes it; t005 and t006 follow; t007's live readings wait for the deploy that carries t003.

### 2026-11-19 — t002: what makes an App change wait, read on production (loopx w5, read-only)

Production Prometheus (3 days kept), Loki and the operator's own `/metrics`, pin `dff52c596d13`, 28 to 29 Apps. Every window is after w21/m421's CPU fix.

- **Steady state is not the problem.** Last 24 h of the App queue: wait p50 0.0001 s, p95 0.05 s, p99 5.8 s; reconcile p50 0.12 s, p95 1.1 s; 58,560 passes (one per App every 41 s, 94% of them `RequeueAfter`); 0.16 of 2 workers busy. Five maintenance toggle pairs on a Starter image service in `bex-canary` flipped the public host in 0.9 to 6.5 s, of which the PATCH itself took 0.3 to 5.7 s and the flip after it at most 0.7 s.
- **The minute-long waits are operator restarts.** Splitting 71 h of `workqueue_queue_duration_seconds{name="app"}` at each operator pod's creation: the 10 minutes after a start hold 7% of the time and 8% of the passes, but **95% of the waits over 10 s** (1,016 of 1,071). Inside them the queue wait p95 is 37 s and p99 87 s; outside them p95 is 0.06 s and 0.034% of passes wait over 10 s. Each start makes 13 to 85 passes wait over 10 s.
- **Why a restart stalls the queue, measured per rollout:**
  1. **No leader for 14 to 21 s.** The stopping manager does not give its lease back (`LeaderElectionReleaseOnCancel` was off), so the next pod waits out the 15 s lease. Loki's `Wait completed, proceeding to shutdown` to the next pod's `Successfully acquired lease`: 53 handovers in 3 days, min 14.3 s, median 16.9 s, max 20.7 s. Nothing reconciles any App in that gap.
  2. **Then a sweep of the whole fleet that a change made in the gap waits behind.** The new manager queues every App from its informer replay at controller-runtime's `LowPriority` (-100); a change made after the start is queued at 0 and jumps it, but one made during the gap arrives inside the replay. Both workers stay busy for about 75 s (11:35:59 to 11:37:15 on 2026-11-19), at 0.5 to 2.5 s a pass (78 to 120 passes, 137 to 150 s of reconcile time, in the 2 minutes after a start), against 0.12 s in steady state. The operator used 0.36 to 0.95 of its 2 cores and was never throttled, so the sweep is bound by the two workers, not CPU. At about 100 Apps the same per-App cost (5.2 worker-seconds) makes it about 4.3 minutes; the steady state would still be under a second at p95 (0.6 of 2 workers busy).
  3. **Every deploy restarts the operator twice.** 50 pod creations in 71 h came in 25 pairs one to three minutes apart: Argo rolls the new digest, then `deploy.yml`'s "roll bex onto the freshly-pushed image" step runs `kubectl rollout restart deploy/bex-controller-manager` (the second ReplicaSet differs only in `kubectl.kubernetes.io/restartedAt`). The step's comment predates digest pinning. The first manager's sweep is cut off by the second's start.
- **So a toggle made at a rollout waits up to about 17 s plus about 75 s, while one made minutes later takes seconds.** Round 181's 80 s and 55 s (w4/603, 2026-11-12 09:09 to 09:12) were not at a rollout: the operator's last handover that morning was at 07:26 (Loki). They were the CPU throttling w21/m421 fixed on 2026-11-16, when the queue's p95 was 95 to 145 s all day. The restart windows are what is left after that fix.
- **Readers of the Builds panels:** `resets()` on the App controller's histograms counts about one per hour per pod. That is controller-runtime's native-histogram reset (`NativeHistogramMinResetDuration` 1 h), not a restart; `controller_runtime_reconcile_total` does not reset with it, and `rate()`/`increase()` are unaffected.
- **Load:** none generated beyond the one Starter fixture (`m370-toggle`, deleted at the end of the run: DELETE 204, then GET 404). The fleet's own restarts were the reproduction.

### 2026-11-19 — t003, t005, t006 (loopx w5)

- **t003, the fix for what t002 measured.** (1) The stopping manager hands its lease back (`withLeaderElection`, `LeaderElectionReleaseOnCancel`), so the next one leads within the election's 2 s retry period instead of after the 15 s lease; envtest measures 15.3 s without it. (2) An App that is mid-change when a manager starts (deleting, an unobserved generation, a build or rollout in flight) is queued at the default priority, ahead of the start's `LowPriority` sweep (`inFlightFirstAtStart`, `internal/controller/startup_priority.go`). On production's fleet today that is 4 of 29 Apps (three failed releases whose generation stays unobserved, one failed rollout behind a serving release), so a real change waits behind at most a few passes. `BEX_APP_RECONCILE_WORKERS` stays 2: with changes out of the sweep, its length no longer reaches a tenant.
- **Not changed here:** the second operator restart per deploy, filed as [w5/196](../196.md).
- **t005:** one comment applied; the envtest lookup's third copy kept (no shared test package). **t006:** the tests above fail against the pre-milestone code; `BexOperatorQueueMetricsMissing` makes an absent queue series loud, with promtool cases for absent and present.

### 2026-11-19 — parked for t007 (loopx w5)

- **Gate:** production's pin must carry `500c5974b` (t003), and 24 hours of readings after that roll (`date gate: 2026-11-20T15:00Z`; the deploy started 13:53Z, and a full deploy takes about 30 minutes).
- **Then t007** (its § Context says how): the 24 h queue reading over a window that starts after the roll, the handovers in Loki (about 2 s once the stopping pod runs t003's binary), a maintenance toggle through an operator rollout within 15 s, the five toggle pairs of DoD 3, the w21/012 append, and the fixtures deleted (GET 404).

### 2026-11-20 — gate check (loopx w5, `/pm unblock`)

- **Affected:** t007. **Source:** verified evidence; no user decision.
- **The gate's checkable forms hold:** production's pin `12716d1d7097` carries `500c5974b` (`git merge-base --is-ancestor`), and 2026-11-20T15:00Z has passed.
- **The 24 h it stood for does not yet exist.** The date was counted from the fix's commit (13:53Z). The first pin carrying it, `c81d2d447bd8`, rolled the operator at 18:07Z on the old image (that pod's container digest is the previous pin's), and the first pod running t003's binary was created at 18:08:52Z and led from 18:09:37Z. A full day after it ends 2026-11-20T18:10Z.
- **So the item stays parked** for the 24 h reading alone (`date gate: 2026-11-20T18:15Z`). The rest of t007 was done in this run (below).

### 2026-11-20 — t007's readings so far (loopx w5)

Production Prometheus and Loki through `scripts/fetch-app-kubeconfig.sh` and port-forwards, as t002 read them. 26 Apps besides the fixture.

- **Queue, the 20.95 h that exist (2026-11-19T18:10Z to 2026-11-20T15:07Z, 9 operator starts):**
  - App queue wait p50 0.00005 s, p95 0.37 s, p99 7.2 s over 59,176 passes. Reconcile p50 0.10 s, p95 1.15 s. 0.20 of 2 workers busy on average.
  - Split at each operator pod's creation as t002 did: the 10 minutes after a start hold 7% of the passes (4,163) and 98% of the waits over 10 s (221 of 226). Inside them p95 15 s and p99 83 s; outside them p95 0.087 s, with 0.009% of passes over 10 s (5 of 55,013; t002 measured 0.06 s and 0.034%).
  - Each start still makes 16 to 32 passes wait over 10 s (t002: 13 to 85). The startup sweep of settled Apps at `LowPriority` is as long as before, which t003 chose to keep. `bex:operator_app_queue_wait_seconds:p95` reads 53 to 82 s for 4 to 6 minutes after each start, and `BexOperatorReconcileQueueSlow` goes pending then without firing (its `for` is 10 minutes).
- **Handovers (Loki, `Wait completed, proceeding to shutdown the manager` to the next pod's `Successfully acquired lease`):**
  - All 10 from a stopping pod on t003's binary took 0.05 to 3.3 s (median 1.8 s): 18:53, 18:54, 19:51, 19:54, 21:13, 21:15 and 22:00 on 11-19, then 04:25, 08:50 and 15:25 on 11-20.
  - The two at 18:07 and 18:09 on 11-19 stopped the old binary and took 21.4 s and 18.2 s (t002: 14.3 to 20.7 s).
  - Since 22:00Z on 11-19, each deploy has started the operator once (w5/196).
- **Five toggle pairs (DoD 3), 15:04Z to 15:05Z:** on a new Starter image web service in `bex-canary` (`m370-toggle`), measured from the PATCH's start. Enabling flipped the public host in 3.8, 9.3, 4.5, 3.5 and 3.3 s, and disabling in 1.3, 1.7, 1.0, 1.3 and 8.7 s: 5 of 5 within 15 s. The two slowest were an 8.2 s PATCH and a 6.0 s flip after the PATCH answered.
- **Through an operator rollout (DoD 2, t003's second half):**
  - The deploy of pin `11b1e8f85f06` started a new operator pod at 15:25:23Z. The old leader stopped at 15:25:59.2Z and the new one led at 15:26:02.5Z (3.3 s). Its App workers started at 15:26:02.8Z, and its sweep held both workers until about 15:27:25Z (depth 21 at 15:26:15Z).
  - From the new pod's creation through 15:28:24Z, the fixture's maintenance mode was toggled 62 times. Every flip came within 15 s, with no PATCH answering anything but 200. The 23 toggles inside the leaderless gap and the sweep took at most 9.6 s to enable and 12.2 s to disable; outside that window the slowest flip was 6.1 s.
  - The toggled App was the first one the new leader reconciled, 0.4 s after its workers started. It then had a pass every 1 to 12 s while the sweep ran.
  - Its slowest flip (12.2 s at 15:26:51Z) came while both workers were in long passes of sweep Apps (three such passes ended between 15:26:57Z and 15:27:02Z). A change goes ahead of the sweep but still waits for one of the two workers to come free.
  - Only enabling depends on the operator: disabling is answered by the activator from its own watch while the Ingress still points at it (`hold.proxyNow`, w4/603).
- **Fixture deleted:** DELETE 204, then GET 404; no App CR is left and the host answers 404. The QA session was logged out.
- **Left for t007:** the 24 h reading over 2026-11-19T18:10Z to 2026-11-20T18:10Z (the queries above, and DoD 2's p95 of 5 s or less), recorded here and appended to [w21/012](../../../w21/done/012.md). The milestone then closes.
