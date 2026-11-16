# w21 · m421 — A deploy starts within seconds and a Blueprint sync never answers 5xx for a cause bex can name: the operator's CPU starvation, the sync's error mapping, the leftover env var

**Worker:** worker21 **Goal:** the minutes-long `queued`/`created` waits and the 500 answers behind w21/012 are traced to their causes on production and fixed, so an image deploy starts within seconds of its create, a Blueprint sync or create answers with a class a client can act on, and the rollout-retry question is answered from Render's own pages. **Status:** in progress (t003, t005, t006 done; t001, t004 shipped and waiting on production verification)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Stop throttling the operator: raise its CPU limit, serve pprof on loopback, alert on throttled platform containers | 50m | — |
| [t002](t002.md) | Profile the operator on production and cut what it spends CPU on | 1h30m | t001 |
| [t003](done/t003.md) | A Blueprint sync or create never answers 500 for an unreachable GitHub or a service still being deleted — **DONE** | 1h | — |
| [t004](t004.md) | A removed manifest variable can be deleted, and `GET …/env-vars` pages as Render does | 45m | — |
| [t005](done/t005.md) | `POST /services/{id}/deploys {}` on a git service: reproduce on production, fix or close — **DONE** | 30m | — |
| [t006](done/t006.md) | A missed rollout deadline: record Render's documented behavior and bex's window in ADR018 — **DONE** | 25m | — |
| [t007](t007.md) | A deploy waiting on the platform says why in its log | 40m | t002 |
| [t008](t008.md) | Render parity | 30m | t003, t004, t005, t007 |
| [t009](t009.md) | Simplify | 20m | t008 |
| [t010](t010.md) | Test coverage | 40m | t008, t009 |
| [t011](t011.md) | Closeout | 30m | t010 |

Total: **11 tasks, about 7h20m**.

## Definition of done

1. On production after the deploy that carries t001 and t002, over a 6 h window: the operator's throttled-period ratio is at most 10% (before: 0.92 to 0.99 in every window for three days), its App queue p95 wait is 5 s or less (before: 28 to 58 s in the windows after w4/665, 95 to 145 s before it), and its median App reconcile is 0.5 s or less (before: 2.1 to 2.5 s).
2. Five Free image web services created one after another in the QA workspace each get their deploy's `startedAt` within 15 s of the create, and every fixture is deleted (DELETE, then GET answers 404).
3. `POST /v1/blueprints`, `POST /v1/blueprints/{id}/sync` and `…/syncs` answer 503 (GitHub unreachable or rate-limited) or 400 (repository, path or access) instead of 500 for every fetch failure, and a sync after a Blueprint-managed service was deleted by hand answers 200 (the service is recreated, as Render does) or a retryable 409, never 500. Each is a test on the real-Postgres suite that fails before the fix.
4. The leftover-variable and list-limit behaviors are matched to Render's documented API or recorded in ADR018 as unsigned divergences; the `{}` deploy is either reproduced and fixed or closed with its production evidence.
5. The missed-rollout question is answered from Render's pages (cited below and in ADR018), as match or as a named unsigned divergence.
6. A deploy that waits on the platform says why in its log, or the wait is shown to be gone (t007).

## Source + Goal linkage

- **Source:** [w21/012](../done/012.md) (promoted 2026-11-16), its 2026-11-12 addenda (QA rounds 179 and 181) and its 2026-11-16 unblock record. Coordinates with [w5/m370](../../w5/blocked/m370/README.md), which measures the same queue wait: its t001 and t004 shipped the series this milestone reads, and its t002 asked for a mock-cluster load reproduction that production now answers (§ Findings). This milestone does not repeat m370's tasks; it records the numbers that bear on m370's definition of done there.
- **Goal linkage:** ADR008 Render parity first: a Render deploy starts at once, a Blueprint sync reports a sync error, not a server error, and the one-click path (w21/008, w21/009) depends on all of it. GOAL item 2 (operation): the operator is a platform component that must not run throttled.
- **Expected outcome:** a new service's deploy starts in seconds, not minutes; a Blueprint sync's failures carry a status a client can branch on; Render's retry behavior is cited, not recalled.
- **Why now:** two QA workstreams keep hitting the lag; the Blueprint one-click flow is the product's front door; the cause turned out to be one platform setting plus one error mapping, both cheap once seen.
- **Render parity:** included (t008): t003 to t007 touch REST, GraphQL, MCP and dashboard answers. Reference product: Render.

## Findings

### 2026-11-16 — production read (loopx w21, read-only; kubeconfig fetched from outside the repo and deleted afterwards)

Prometheus keeps 3 days (`--storage.tsdb.retention.time=3d`), Loki 30 days of logs.

**The queue stall is the operator, and the operator is CPU-throttled.**

- **Where the time is not.**
  - **bex-api's `ReconcileOnce`** (`bex_controlplane_reconcile_pass_seconds`): p50 0.6 s, p95 1.5 s, p99 3.8 s, never over 60 s, over 17,036 passes in 3 days. The round 179 hypothesis (one slow serial pass delays every new App) is false.
  - **The builder's queue** (`bex_build_queue_seconds`): p50 2.5 s, p95 4.8 s, oldest 215 s in 3 days; held builds (`admission` 7, `datastore` 1 at the peak) waited p50 90 s, p95 271 s, oldest 209 s.
  - **The kube-apiserver:** GET 2.6 ms, POST 6.4 ms, LIST 5.4 ms on average; etcd WAL fsync p99 under 5 ms. `rest.Config` client-side rate limiting is off (controller-runtime v0.25.1 sets `QPS = -1`, `config.go:101-105`), so it is not client throttling either.
- **Where it is.** The operator's App controller has two workers (`BEX_APP_RECONCILE_WORKERS=2`) and 27 Apps. Over the last day it ran 47,129 passes (0.55 a second) at a median 2.5 s each, which is 1.4 to 1.5 busy workers of 2 in every 6 h window since 2026-11-15 04:00 UTC. The container has `limits.cpu: 500m` on a 4-CPU node and used 0.44 to 0.50 cores throughout, throttled in **92% to 99% of its CFS periods** (`container_cpu_cfs_throttled_periods_total`), in every one of the last 15 operator pods. A pass issues about 24 API reads (80,241 GETs in 100 min), so a throttle stall of 50 to 100 ms per read is the 2.5 s median.
- **Queue wait by 6 h window** (`workqueue_queue_duration_seconds{name="app"}`):

  | window (UTC) | p50 | p95 | p95 reconcile | busy workers |
  | --- | --- | --- | --- | --- |
  | 13 Nov 10:00 to 14 Nov 22:00 (before w4/665) | 7 to 41 s | 95 to 145 s | 19 s | 1.85 to 1.98 |
  | 15 Nov 04:00 to 16 Nov 08:00 (after w4/665) | 0.5 to 0.85 s | 28 to 58 s | 6 s | 1.40 to 1.50 |

  Worst 5-minute p95 in the three days: 910 s. Depth peaked at 37 (priority -100) and 25 (priority 0). w4/665 removed the 15 s port sweeps from the worker and the p95 reconcile fell from 19 s to 6 s, but the median did not move (2.1 to 2.5 s): what is left is throttling.
- **Other containers.** bex-api (limit 200m, uses 0.04 to 0.07 cores, throttled 3 to 10%), objectgw, ssh-gateway and static-server are not throttled. The operator is the one platform container that is.
- **Falsified on the way:** a port-scan loop (3 "ran out of time" lines in 3 days), client-side rate limiting, a slow apiserver or etcd, bex-api's projection.

**The 20 to 35 minute `queued` waits of 2026-11-08: the likeliest cause is gone, and the data to prove it is too.** A four-service Blueprint on a workspace with the per-workspace build cap of 2 builds in waves ([ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md) D6): the cap was retired on 2026-11-10 (`w6/m306`), and builds of another workspace started at once because they were not capped. Held builds are narrated by `waitLine` as "Waiting for platform build capacity" (`BuildQueued`). The operator's queue wait was then 95 to 145 s at p95 and 7 to 41 s at p50, so it added minutes to a multi-wave Blueprint, not 20.

**The sync and create 500s (Loki, bex-api, 30 days).** `bex-api: internal error:` lines on 2026-11-08 name two causes:

1. `unavailable: Could not reach GitHub to read the Blueprint file. Try again in a few minutes.` at 10:52:47, 10:53:32 and 10:54:28. It is the plain error `blueprintFetchFailure` builds for a sync's history (`apps/blueprint.go:393`), and `failSource` returns it to the caller, so every sync fetch failure (unreachable, rate-limited, repository or path not found, access denied) answers 500. The create path was fixed in w8/134 (`blueprintCreateFetchError`: 400 or 503); the sync path never was.
2. `object is being deleted: apps.app.bex.co "<ns>-hello-node-2gzk" already exists`, 17 times from 2026-11-08 13:09 to 13:13 and twice on 2026-11-09 00:43 to 00:44: a sync that recreates a service whose App is still terminating (the operator's finalizer cleans registry credentials and artifacts first). This is "deleting a Blueprint-managed service by hand made the next sync 500". Render recreates a deleted resource on the next sync ([Blueprints](https://render.com/docs/infrastructure-as-code): "Manually deleting a resource does not remove it from your Blueprint. … Render detects that a defined resource is missing and recreates it").

In the last 3 days the origin answered **no 5xx on any Blueprint route** (`bex_api_http_requests_total`), and POST `/v1/blueprints` and `/syncs` finish in 2.5 s and 1 s at the maximum. The 502s of 2026-11-08 were not logged by the origin and no edge log is kept in Loki, so they cannot be read back; they are consistent with the bex-api pods rolling during the session and are not chased.

**Render's documented behavior (read 2026-11-16).**

- [Deploys](https://render.com/docs/deploys): "If any command fails or times out, the entire deploy fails" and the service "continues running its most recent successful deploy"; "Only one deploy can run at a time per service"; the Wait policy lets the in-progress deploy finish, then runs the most recently triggered one. The page states no start latency (it says a deploy starts "as soon as you push") and **mentions no automatic retry** of a failed deploy.
- [Health checks](https://render.com/docs/health-checks): "If this condition is not met within 15 minutes, Render cancels the deploy"; a running instance failing for 15 s stops receiving traffic and after 60 s is restarted. No retry of the cancelled deploy.
- So a rollout that misses its window is cancelled, never retried, and a new deploy is the recovery. bex's operator budget is that same 900 s window and the control plane closes the row at 18 min (`reconciler.go:123`): the observed "about 17 minutes" then `update_failed` is Render's behavior, not a bug. The note's bullet 3 is **already met**; t006 records it in ADR018.

### 2026-11-16 — the other bullets (loopx w21)

- **`POST /services/{id}/deploys {}` replaying the previous image: does not reproduce (t005).** A public-repo Docker service in `bex-canary` (`bex-co/bex`, `examples/hello-node`) went `live` on its first deploy at commit `00d9636661` (the branch head at the time). Two plain `{}` triggers right after answered 201 with `commit: 00d9636661`, `status: created`, started 4 s and 16 s later (the 16 s is the operator's queue under the CPU throttle) and went `live` in about 100 s each: each rebuilt from source at the branch head. The note's `commit: null` is what `Trigger` records when it cannot resolve the branch head (`deploys/service.go`, "or `""` when it cannot be resolved"), and bex-api logged `Could not reach GitHub` three times in 2026-11-08 10:52 to 10:54 UTC; the operator then builds the branch head itself. The service was deleted (204, then 404).
- **`DELETE`/`PUT` of a variable the manifest dropped answered 409 (t004): reproduced and fixed.** On production a Blueprint synced with `MESSAGE` and `KEEPME`, then a sync whose manifest dropped `MESSAGE`: the variable stayed on the service (as on Render, which "preserves existing environment variables, even if you omit them from the Blueprint file", [Blueprint spec](https://render.com/docs/blueprint-spec)) and `DELETE` and `PUT` answered 409 `ENV_VAR_MANIFEST_MANAGED` ("declared in this service's render.yaml manifest"). Fix: the sync records the names its manifest declares (`bex.co/blueprint-env`), and an undeclared literal reads and writes as an ordinary variable. The list default of 20 is Render's own: the [list endpoint](https://api-docs.render.com/reference/get-env-vars-for-service) documents `limit` default 20, maximum 100. Fixtures deleted (204, then 404).
- **`object is being deleted … already exists` on a sync after a hand-delete: already fixed (t003).** `w2/m365` (shipped 2026-11-12 to 2026-11-14) gives a re-created service its own App object while the deleted one finishes. A Blueprint synced two seconds after the hand-delete of its service answered 200 and recreated the service, for a static site and for a Docker service (`hello-node`, the Nov 8 log's service), on production today. What was left of the sync's 500 was the unclassified GitHub fetch failure, fixed in `ce80e404d`.
- **The other 500 of 2026-11-08, `recording redeploy: deploy: not found`, 09:00:07, 09:00:32 and 09:01:14 (3 lines), is the same hand-delete race from the other side.** A sync that found the deleted service's App still terminating treated it as an existing service and tried to record a redeploy against a store row that was already gone (`recordBlueprintRedeploy`, `apps/deploy.go:4390`). The sync's workspace snapshot has skipped an App Kubernetes is deleting since `49ffa1735` (2026-11-07, `w2/m256` t014), and a re-created name gets its own App object since `18b9e89d5` (`w2/m365`, 2026-11-12), so neither 500 can recur through the path the log shows; today's replays above are the evidence. The half-created `beancount-api` after a `POST /blueprints` 500 has no trace in the 30 days of bex-api logs beyond these windows and is not chased.
- **Deploy ordering found while shipping t001: confirmed on production, already filed as [w8/331](../../w8/done/331.md).** The new `--pprof-bind-address` reached the manager's manifest through Argo at the push (09:00 UTC) while the image pin landed about 35 minutes later, so the new ReplicaSet crash-looped (`flag provided but not defined`, exit 2, 8 restarts) until the pin synced, and the CPU-limit raise did not apply until then; the old pod kept serving (`maxUnavailable: 0`). The w21 note filed for it was withdrawn as a duplicate.

### 2026-11-16 — t001 on production, and the profile that t001's pprof made possible (loopx w21)

- **Rollout.** `f114fdd9f` reached the manifest at 09:00 UTC and the image pin at about 09:24, so the new pod crash-looped in between (w8/331). The operator ran on the new image from 09:25 UTC: limit 2 CPUs, no throttling (`container_cpu_cfs_throttled_periods_total` ratio 0.0 over the first 4 minutes, from 0.92 to 0.99 for three days).
- **Effect of t001 alone, first 4 minutes (`workqueue_queue_duration_seconds{name="app"}`, `controller_runtime_reconcile_time_seconds{controller="app"}`):**

  | | before (6 h windows since 15 Nov 04:00) | after (4 min, pod started 09:25) |
  | --- | --- | --- |
  | queue wait p50 | 0.5 to 0.85 s | 0.0 s |
  | queue wait p95 | 28 to 58 s | 0.76 s |
  | queue wait p99 | about 412 s (3 days) | 7.1 s |
  | reconcile p50 | 2.1 to 2.5 s | 0.54 s |
  | reconcile p95 | 6.0 s | 2.2 s |
  | busy workers (of 2) | 1.40 to 1.50 | 0.51 |
  | CPU used | 0.44 to 0.50 cores (capped) | 0.56 cores |

  A 6 h window follows t002's deploy; these are the first minutes.
- **The profile (60 s, steady state, pod 5 minutes old, 33.8 CPU-seconds = 0.56 cores):** 92.7% of it is `portscan.scan` (the private-network listener sweep, 512 goroutines dialing ports of each Ready service's pod for its whole 15 s timeout, every service every 2 minutes), almost all of it connect syscalls (`syscall.Syscall6` 70% flat, `net.(*Dialer).DialContext` 87% cumulative). A 14 s profile of the pod's startup shows the same (75% in `portscan.scan`) plus 15% in bcrypt for the first pass over each registry credential (memoized afterwards, `registry/htpasswd.go`). That is what the operator was spending its 500m on, and why w4/665's move of the sweep off the worker, which fixed the worker saturation, left the CPU in the same container as the workers. No other function is over 1% of the steady-state profile.
