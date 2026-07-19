# ADR034 — Scalable build pipeline: ephemeral builders on elastic Kubernetes capacity

**Status:** Accepted · 2026-07-15

> **Security closure (2026-07-19):** w2/m59 closed [ADR039 O-01/O-02](ADR039-operator-audit-and-platform-reuse.md): source clone, BuildKit, Skopeo push, and Cosign run as serial phases in `bex-build`; BuildKit keeps its process sandbox inside a Pod user namespace and never receives the output/signing credentials; service-account tokens, private/platform egress, and platform-node placement are denied. `scripts/verify-build-isolation.sh` passed the 38-control live matrix described in ADR022.

## Context

bex builds git-backed services inside the app cluster. Dockerfile and Render-native runtime builds use [BuildKit](https://github.com/moby/buildkit) Jobs confined by Kubernetes Pod user namespaces; the explicit `builder: buildpack` extension uses kpack. Each pipeline pushes an immutable generation image to Zot before the operator rolls out the workload. This matches Render's publicly documented use of [BuildKit for Dockerfile deploys](https://render.com/docs/docker) without assuming that Render's undisclosed native-runtime implementation has the same internal shape.

The first implementation called `build.Build` synchronously from an App reconcile. The function creates or adopts the deterministic build Job, then polls it until success, failure, or the 20-minute deadline. With controller-runtime's default single App worker, one long build therefore occupied the only reconcile worker. A second App could remain waiting even when Kubernetes had enough CPU and memory to run another build.

This exposed two different scaling problems that must not be conflated:

| Layer | Unit | Responsibility | Adds build compute? |
| --- | --- | --- | --- |
| Control | App reconcile worker in the operator process | Admit, create/adopt, observe, and finish one App reconciliation | No |
| Execution | BuildKit Job/Pod or kpack build Pod | Clone, compile, assemble, and push one image | It consumes compute but does not create it |
| Capacity | Kubernetes node / physical machine | Supplies CPU, memory, disk, and network to build Pods | Yes |

There is no persistent `builder worker` Deployment in the current design. One admitted build creates one ephemeral builder Pod. Increasing reconcile workers can create more of those Pods; it cannot make the underlying machines faster or larger.

## Decision

### 1. Keep build execution ephemeral and Kubernetes-native

Each admitted source build runs as an isolated Kubernetes workload:

- Dockerfile and native-runtime builds run an ephemeral BuildKit Job. A clone init container stages source, BuildKit exports an OCI archive under its default process sandbox, Skopeo pushes with the App-scoped registry credential, and optional Cosign signing runs last.
- The buildpack extension remains a kpack Image/Build.
- A successful builder pushes an immutable generation image to Zot; workloads pull by the resolved image reference.
- Build names are deterministic per App generation, so a manager restart adopts the existing work instead of starting a duplicate.
- Superseded builds are newest-wins per App: a newer revision cancels the older active build before starting.

We will not introduce a shared, long-lived BuildKit daemon as the default worker pool. Ephemeral builders preserve failure isolation, make Kubernetes resource accounting and cancellation natural, and avoid making a daemon filesystem or local cache a cross-tenant trust boundary.

### 2. Treat dispatch concurrency, tenant admission, and machine capacity as separate controls

The baseline production configuration is two parallel builds:

| Control | Setting | Baseline | Scope |
| --- | --- | --- | --- |
| App dispatch/observation | `BEX_APP_RECONCILE_WORKERS` | `2` | Maximum concurrent App reconcile loops in the active operator manager |
| Tenant admission | `BEX_MAX_CONCURRENT_BUILDS` | `2` | Maximum active build Jobs for one labeled workspace; `0` means unlimited |
| Physical capacity | schedulable node CPU/memory | One schedulable 4 GiB slot per admitted build | Cluster-wide scheduler and node-pool constraint |

With the current synchronous reconciler, the practical global build concurrency is bounded by the App worker count. For a particular workspace it is also bounded by its workspace cap. In simplified form:

```text
running builds <= min(reconcile worker slots, workspace admission slots, schedulable Pod capacity)
```

The variables have intentionally different names because they control different things:

- `BEX_APP_RECONCILE_WORKERS` is a controller-runtime concurrency setting. It also affects non-build App reconciles and consumes little compute by itself.
- `BEX_MAX_CONCURRENT_BUILDS` is an admission/fairness ceiling. It prevents one workspace from occupying every admitted build slot, but does not create a worker or a machine.

Values must be raised deliberately and together with a capacity check. A high reconcile-worker count with no admission limit can create a build stampede. A high workspace cap with one reconcile worker still serializes builds.

### 3. Scale Pods and machines independently

Adding build Pods can improve throughput without adding machines only while existing nodes have spare allocatable CPU, memory, ephemeral storage, and network bandwidth. Once that headroom is exhausted, extra Pods either contend on the same node or remain Pending; they do not manufacture compute.

The BuildKit and kpack builders request `500m` CPU and 4 GiB memory and are limited to 4 CPU and 6 GiB. Kubernetes schedules from requests, not likely peak usage. On the baseline 8 GB tenant nodes, two build requests therefore cannot fit together: Pending builders make the real capacity shortage visible to Cluster Autoscaler instead of silently competing on one host. The 6 GiB limit leaves memory for kubelet and node daemons; advertising an 8 GiB container limit on an 8 GB machine offered protection only on paper.

This sizing was corrected after a production incident on 2026-07-16/17: two builders admitted under the former 1 GiB-request/8 GiB-limit shape consumed about 4.07 GB and 1.87 GB while total container working set on the tenant node reached about 7.15 GB. The kernel reported `SystemOOM`, the node became unreachable, and MachineHealthCheck replaced it. The recovered builds completed, but recovery is not admission control; the 4 GiB request prevents the same co-location pattern, and the tenant pool's one-to-three autoscaling range can supply a separate node for each of the two admitted builds.

Production build Pods contain untrusted tenant code. Every direct execution Pod explicitly selects `bex.co/pool=tenant`; platform taints are defense in depth rather than the only placement boundary. They must never be made eligible for the platform pool merely to gain capacity.

The scaling sequence is:

1. Increase logical parallelism only up to the capacity already available.
2. Let unschedulable build Pods become visible as Pending Pods.
3. Let Cluster Autoscaler and Cluster API add machines within the tenant pool's declared minimum and maximum.
4. When sustained build load would interfere with serving workloads, introduce a dedicated untrusted `build` node pool with its own label/taint, place build Pods there, and give that pool an independent autoscaling range.

`BEX_BUILD_NAMESPACE` is a namespace and credential-placement boundary, not a node-placement control. Moving Jobs to `bex-system` does not move them to platform machines.

### 4. Preserve bounded, fair, restart-safe behavior

The pipeline keeps these invariants at every scale:

- **Newest wins per App.** At most one nonterminal build for an App should survive a new revision.
- **Per-workspace fairness.** A configured tenant limit queues excess builds as `BuildQueued` and retries admission instead of failing the deploy.
- **Bounded execution.** Build Jobs have a deadline and bounded retry behavior; completed Jobs are reaped after the log-collection window.
- **Idempotent recovery.** Deterministic names let reconciliation adopt Jobs after operator restart.
- **Immutable handoff.** Deploy and pre-deploy steps consume the image produced for that exact generation.
- **No credential widening.** Increasing concurrency must not broaden private-git, private-base, App-repository, or signing credential placement. The serial phase topology and live adversarial assertions that closed ADR039 O-01 are regression requirements.

The current per-workspace gate counts active Jobs and kpack Images before creating the next build. It is a best-effort fairness guard, not an atomic semaphore: two reconcile workers can both observe an open slot before either creates its Job. With the baseline global worker count and workspace cap both set to two, the global worker ceiling prevents an overshoot above two; a smaller workspace cap can transiently overshoot at the list/create boundary. Strict plan enforcement, more workers, or dispatch sharded across managers or clusters requires durable, atomic admission.

### 5. Decouple reconcile workers from build duration as the next scaling step

Two synchronous workers are the accepted near-term baseline, not the final queue architecture. Before raising concurrency substantially, App reconciliation will become non-blocking:

1. validate and admit the requested build;
2. create or adopt the Job/Image;
3. record `Building` and return from reconcile;
4. reconcile again from owned-resource events or a bounded requeue;
5. on terminal build status, resolve the image and continue the rollout.

After this change, `BEX_APP_RECONCILE_WORKERS` controls control-plane responsiveness only. Build parallelism will be controlled by explicit admission and Kubernetes capacity, so a 20-minute build no longer leases a reconcile goroutine for 20 minutes.

The Kubernetes Job remains the execution contract. A queueing layer such as Kueue can later be inserted for resource-aware regional quotas, priorities, or fair sharing without replacing BuildKit or changing the image handoff. We will not add that dependency for the two-build baseline. Before multi-manager or multi-cluster dispatch, the follow-up design must select and document one atomic admission authority rather than extending the current list-then-create check.

### 6. Prefer remote, tenant-scoped cache over node-local cache

BuildKit Jobs are currently ephemeral and configure no `cache-from`/`cache-to`, so every build is a clean build. Adding machines increases parallel throughput but does not reduce repeated work.

If build latency or registry traffic becomes the limiting factor, cache will be added as an explicit remote OCI cache in Zot or dedicated object storage. Cache keys and authorization must be scoped so one workspace cannot read another workspace's private build layers. Node `hostPath` caches and a shared daemon cache are rejected as the default because they couple correctness and tenant isolation to machine lifetime.

Cache objects need an independent retention and garbage-collection policy from immutable deploy images. A deploy must remain reproducible when the cache is empty or corrupt; cache loss may make a build slower, never change its result.

## Capacity model and diagnosis

The build pipeline should be diagnosed from the narrowest blocked layer outward:

| Observation | Bottleneck | Correct response |
| --- | --- | --- |
| Another App is not dispatched while all App workers are occupied by long builds | Reconcile-worker saturation | Raise the small baseline or complete the non-blocking reconcile change |
| App reports `BuildQueued` and no Job exists | Workspace admission cap | Wait, cancel obsolete work, or deliberately change the tenant limit |
| Build Job exists but Pod is `Pending` / `FailedScheduling` | Physical capacity or placement | Inspect requests, taints, pool maximum, and Cluster Autoscaler; add/resize machines if necessary |
| Multiple build Pods are Running but all are slow | CPU, memory, disk, registry, or network contention | Measure node and registry saturation; add capacity or tune requests before adding more Pods |
| Build completes but rollout does not advance | Reconcile, image push/resolution, pre-deploy, or health gate | Inspect App conditions and the later pipeline stage; builder capacity is no longer the cause |

Required operational signals are queue wait time, active/queued build count by workspace, build duration and outcome, Pod scheduling latency, requested versus actual CPU/memory, registry push latency, and node-pool saturation. Build logs continue to flow to Loki as `type=build`, and completed Job timestamps remain the source for `build_seconds` usage metering.

## Alternatives rejected

- **Only add operator replicas.** Leader election leaves one active controller manager, and replicas do not supply build CPU. Active-active dispatch would also require atomic admission first.
- **Only increase reconcile workers.** This creates more Pods but cannot exceed physical capacity and can make contention worse.
- **Only add machines.** Spare nodes do not help when one synchronous reconcile worker admits only one build.
- **Use `BEX_APP_CB_WORKERS`.** There is no separate callback or builder-worker pool to configure. The actual controller-runtime setting is App reconcile concurrency, hence `BEX_APP_RECONCILE_WORKERS`.
- **Run builds on platform nodes.** Build steps execute untrusted tenant code and must not share the platform trust boundary.
- **Unlimited first-come-first-served builds.** One workspace could monopolize compute and cause an uncontrolled autoscaling/cost spike.

## Consequences

Two independent Apps can now build concurrently when both admission and node capacity permit it. The design can use spare capacity on existing machines and can grow onto new machines through the existing Kubernetes/CAPI autoscaling boundary.

The near-term implementation still couples one reconcile worker to one running build, so two long builds can occupy both baseline workers. The per-workspace list/count admission is also intentionally best-effort, not the final strict or distributed queue. These limitations are explicit triggers for the non-blocking and atomic-admission follow-ups, rather than reasons to hide the present two-build improvement behind a misleading builder-worker setting.
