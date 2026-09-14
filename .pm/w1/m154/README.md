# w1 · m154 — A deploy or config-change rollout drops live requests with `502 Bad Gateway` at the pod switchover

**Worker:** worker1 **Goal:** replacing a web service's pod never fails a request that reaches it. The old pod keeps serving through a drain window after it leaves the load balancer, then gets `SIGTERM`, and only then `maxShutdownDelaySeconds` before `SIGKILL`. That is Render's order, and ADR004's "zero-downtime by construction" claim becomes true in practice as well as on paper. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                       | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Tenant pods drain before SIGTERM: a native `preStop.sleep`, with the grace period widened so the shutdown delay still counts from SIGTERM | 40m | —          |
| t002 | Blast radius: every path that removes a serving pod, and the controls that must not change                  | 40m | t001       |
| t003 | Render parity                                                                                               | 25m | t002       |
| t004 | Simplify                                                                                                    | 15m | t003       |
| t005 | Test coverage                                                                                               | 40m | t003       |
| t006 | Closeout                                                                                                    | 10m | t005       |

## Definition of done

Repeat on a throwaway free web service: `bex-co/bex` `examples/hello-go`, docker, env `MESSAGE=v1`, live and returning `200`. `examples/hello-go/main.go` has no `SIGTERM` handling, like most tenant apps. Only states observed at filing time are listed:

- **A config-change rollout under constant traffic returns no error.**
  1. Start sampling the service URL continuously (`curl` in a loop with a 0.2 s sleep, logging status, body and millisecond timestamp).
  2. Run `PUT /v1/services/<srv>/env-vars/MESSAGE {"value":"v2"}`.
  3. Keep sampling until the new `config_change` deploy is `live`, plus 60 s.

  Every response is the app's own `200`, with the old body and then the new one, and no `502`/`503`. At filing time each of two runs dropped exactly one request, with `502 Bad Gateway` between two responses from the new pod: 19:35:55.876 `200 app-is-up-3`, 19:35:56.729 `502 Bad Gateway`, 19:35:57.724 `200 app-is-up-3` (209 samples, 1 non-200). The first run at 1 Hz saw `502` at 19:31:51 among 150 samples.
- **The new value is served once the deploy is live.** At filing time this held (19:32:10 `app-is-up-2 [200]`), and the fix must keep it.

The other termination paths (restart, scale-in, autoscale, hibernate, private-service in-cluster traffic) are t002 work to verify. They are not assertions here, because none of them was probed.

## Evidence (probes run 2026-09-14, production, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`)

- **Fixture:** `qa-20260914-maint` (`srv-dak4kqq6m8ac739r63ng`), a free web service built from `examples/hello-go` (docker) with `MESSAGE=app-is-up`.
  - Created 19:26:35Z.
  - `healthCheckPath` was set to `/healthz` at 19:30:00, so both runs were in HTTP readiness mode.
  - Deleted 19:38:05Z (`DELETE` → `204`, then `GET` → `404`).

1. **Run 1**, sampled every 1 s:

   ```text
   19:30:38.996Z PUT /v1/services/srv-dak4kqq6m8ac739r63ng/env-vars/MESSAGE {"value":"app-is-up-2"} → 200
   dep-dak4mni6m8ac739r63u0 (config_change): 19:30:43 created · 19:30:47 build_in_progress · 19:31:49 update_in_progress · 19:32:02 live
   curl / every 1 s from 19:30:17 → 200 … 19:31:51 502 (sample 62) · 19:31:53 200 … end 19:34:10 (150 samples)
   19:32:10 curl → "app-is-up-2" [200]
   ```

2. **Run 2**, sampled every ~0.8 s; only status and body changes are logged:

   ```text
   19:34:43.090Z PUT …/env-vars/MESSAGE {"value":"app-is-up-3"} → 200
   deploy …f64km0 (config_change): 19:34:46 created · 19:34:49 queued · 19:34:59 build_in_progress · 19:35:48 update_in_progress · 19:36:01.6 live
   19:34:33.016 200 app-is-up-2
   19:35:55.876 200 app-is-up-3     ← the new pod is serving
   19:35:56.729 502 Bad Gateway     ← one request still routed to the terminating old pod
   19:35:57.724 200 app-is-up-3
   end samples=209 non200=1
   ```

3. **Render's contract** (`render.com/docs/deploys`, fetched 2026-09-14): "Render also updates its networking configuration so that your _new_ instance begins receiving all incoming traffic… After 60 seconds, Render sends a `SIGTERM` signal to your app's process on the _original_ instance… If the app doesn't exit within the shutdown delay (default 30 seconds), Render sends a `SIGKILL` signal."
4. **bex's own contract** (`docs/ADR004-app-deployment.md:223`): "A rolling update is zero-downtime by construction: `replicas: 1` gives `maxUnavailable: 25% → 0` and `maxSurge: 25% → 1`, so Kubernetes starts the new Pod and waits for `PodReady` before the old one is torn down." That holds for readiness gating, but not for the order in which the old pod is torn down.

## Root cause

- **Tenant pods have no drain.** The container projection sets startup, readiness and liveness probes (`lego/operator/internal/controller/deployment_projection.go:252-271`) and the termination grace period from `maxShutdownDelaySeconds` (`:343`, via `app_controller.go:2484-2490`). It sets no `lifecycle.preStop`.
  - `grep -rn preStop lego/operator` finds 0 hits.
  - In the whole repo the only 2 hits are platform workloads: `dashboard/deploy/deployment.yaml:108-116` and `lego/backend/internal/serve/serve.go:18-27`.
- **The mechanism.**
  1. On the rolling update's last step the old Pod is deleted.
  2. The kubelet sends `SIGTERM` immediately.
  3. The Pod's endpoint removal reaches Traefik a moment later. Traefik load-balances across pod endpoints; the operator sets no Traefik `nativelb` annotation, 0 hits.
  4. A process that exits on `SIGTERM` has closed its listener by then (`examples/hello-go/main.go` calls `http.ListenAndServe` with no signal handling), so a request Traefik still routes there gets `502`.
- **bex already hit this on its own dashboard.** "the Node server exits the instant SIGTERM lands, but Traefik keeps routing to a terminating pod for a few seconds (measured 2026-07-18: ~3s of 502s on every roll). preStop delays the SIGTERM…" (`dashboard/deploy/deployment.yaml:108-112`, `w1/m52`, fixed with `preStop: exec sleep 10`). bex-api got an in-process drain because its distroless image has no shell (`serve.go:18-27`). Neither fix was ever applied to tenant pods.
- **Why a native sleep.**
  - Tenant images are arbitrary (scratch, distroless, Alpine), so `exec sleep` cannot be relied on.
  - Kubernetes' native `lifecycle.preStop.sleep` needs no binary in the image. It is available in the operator's pinned `k8s.io/api v0.35.0` (`lego/operator/go.mod:17`, `corev1.SleepAction`), and the production cluster runs v1.34.9 (`infra/clusterapi/overlays/hetzner-caph/cluster.yaml:858`).
  - Kubernetes counts `preStop` time inside `terminationGracePeriodSeconds`, so the grace period must become `drain + maxShutdownDelaySeconds` for the user's delay to keep meaning "after SIGTERM".

## Blast radius

- **Web services.** Every web service whose pod is replaced while it takes traffic, platform-wide, on every:
  - deploy;
  - config-change roll (env var, secret file, env group);
  - restart;
  - spec change that rolls the pod.

  Each roll drops roughly one sampling interval of requests. Env-var saves are frequent, so this is not rare.
- **Traced, not probed** (t002):
  - manual scale-in;
  - autoscaler scale-in;
  - node drain and Cluster Autoscaler consolidation (ADR004 § Rollout headroom);
  - hibernation's final scale-to-0;
  - private services' in-cluster callers, through the ClusterIP/kube-proxy path.
- **Controls that must not change:**
  - Disk-attached services use `Recreate` (`deployment_projection.go:336-337`), whose downtime is documented ("Attaching a disk disables zero-downtime deploys for the service.").
  - Workers take no inbound traffic; decide whether they get the drain at all.
  - Cron jobs, pre-deploy Jobs and static sites (the shared static-server) are not tenant Deployments behind Traefik.

## Adjacent classes

- **Apps that handle SIGTERM** gracefully still drain correctly; they simply receive `SIGTERM` after the drain.
- **Hibernation** must not make a waking request wait on a sleeping old pod. Confirm the Ingress switches to the activator before the scale-to-0 (`ingressBackend`, `app_controller.go:2314-2323`).
- **Suspend** takes a service offline by design (w1/094). A drain there only delays the 503.
- **Rollout duration and node headroom** grow by the drain length. The surge pod holds the headroom placeholder (`deploy/gitops/base/tenant-headroom.yaml`) longer, so pick a drain long enough for endpoint propagation (a few seconds measured) and record why.
- **Validation.** `maxShutdownDelaySeconds` (1–300) validation is unchanged. The widened grace period is platform-internal, and the REST value still reads back as the user set it.

## Unverified (reasoned, not probed this run)

- **Scope of the probe.** Only one free web service was probed: `hello-go`, 1 replica, HTTP health-check mode, config-change rolls. Restart, scale-in, autoscale, TCP health mode, private services and multi-replica services were not.
- **The routing path.** That Traefik routes straight to pod endpoints is read from the missing `nativelb` annotation in the operator, not from a Traefik configuration dump.
- **The propagation lag** itself was not measured. One failed sample per roll at ~0.8 s sampling bounds it at roughly 1–2 s.
- **An earlier contrary result.** `w6/done/m51` (2026-08-23) recorded a manual Restart's zero-downtime claim as a "non-issue". Its sampling cadence isn't recorded, so it may have been too coarse to catch a one-second window.

## Dedupe

- `grep -rli 'zero-downtime|preStop|502 during|during rollout|rolling update' .pm` hits only unrelated items:
  - `DO_NOT_DO.md:21` (disks);
  - `w2/done/m70` (events out of scope);
  - `w7/done/m80` (health-check probe split);
  - `w6/done/m51` (restart, see Unverified);
  - `w9/done/m44`;
  - `w1/done/037` (the platform bex-api/dashboard drain from `w1/m52`, the precedent this milestone extends to tenants).
- No open item covers this, and no `.pm/DO_NOT_DO.md` entry applies.
- Not fixed on `main` as of `45e341e9c`: `git log -S PreStop -- lego/operator` is empty.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-14 pass 28, journey 6 (the zero-downtime claim). It came up while checking health-check and config-change behavior on a free web service, after the maintenance-mode (paid-only; the free plan refuses it honestly) and custom-domain journeys came back clean.
- **Goal linkage:** `docs/ADR004-app-deployment.md` § Rollout headroom's zero-downtime contract, and Render parity for the deploy lifecycle (`render.com/docs/deploys`).
- **Expected outcome:** a tenant can deploy, or save an environment variable, under live traffic without any visitor seeing a `502`.
- **Why now:**
  - Every web service drops requests on every roll, and config saves roll the pod.
  - bex measured and fixed exactly this failure for its own dashboard two months ago, so the tenant gap is a known, bounded fix rather than open research.
- **Render parity:** included. The deploy lifecycle is user-facing behavior, and `maxShutdownDelaySeconds` is described on REST, GraphQL, MCP and the dashboard ("Wait 1–300 seconds after SIGTERM before force-stopping the process (default 30)"). Those descriptions must stay true after the grace-period change.
