# builder-issues — build plane: ranked defects, capacity decision, proposed milestones

**Status: MATERIALIZED 2026-08-17** as `w7/m82` (P0/P1/P7) · `w7/m83` (P2/P3/P4) · `w7/m84` (P5) · `w7/m85` (P8) · `w7/m86` (P9), plus inbox notes `029` (egress-meter placement) · `030` (ClusterBuilder staleness + re-resolve cadence) · `031` (serving headroom, P6). `024` and `028` were retired to `done/`. This document stays as the evidence and reasoning record behind them.

**Working document**, not a numbered inbox note (same pattern as `.pm/w6/RESEARCH-workspaces.md`). Produced by `/pm-brainstorm for w7 to work on` on 2026-08-17 against `559e664c`, then **corrected the same day against live hetzner-prod measurements**. Nothing here is materialized on the board yet.

Evidence classes used below:

- **[measured]** — read from hetzner-prod via `kubectl` (admin kubeconfig fetched with `scripts/fetch-app-kubeconfig.sh`).
- **[code]** — verified by reading the repo at `559e664c`.
- **[refuted]** — asserted in an earlier revision of this document and disproven by measurement. Kept deliberately: the reasoning trail matters more than looking right.

---

## 1. Why this document exists

`docs/ADR060-build-worker-reliability-and-performance.md` has an eight-decision rollout. **D1 shipped** (`w2/m72`, 2026-08-16) and **D8 was partially executed live** (2026-08-15). **D2, D3, D4, the D5 remainder, D6 and D7 have no owner anywhere on the board** — checked across every workstream including `done/`.

w7 also holds two unresolved build-plane notes: `.pm/w7/024.md` (build pods cannot schedule; its live check was never run — **now run, see §4**) and `.pm/w7/028.md` (Deployment `CreateOrUpdate` PUTs every reconcile).

w7's capacity is free: its only open milestone is `m77`, whose sole remaining task `t007` is hard-blocked on production credentials plus authorization to migrate live forum data.

---

## 2. Ranked problem list

Ranked by: harm actually occurring now > blast radius > whether it blocks measuring everything else > cost to fix.

| # | Problem | Evidence | Harm today | Fix cost | Milestone |
| --- | --- | --- | --- | --- | --- |
| **P0** | Deterministic build failures are retried; every build failure is unattributable | [measured] §3.1 | **Active.** Wasted 7Gi node-exclusive slots on retries that cannot succeed; tenants cannot tell their error from ours | ~4h | **m82** |
| **P1** | A node drain mid-build fails the tenant's deploy; the workaround blocks legitimate consolidation | [code] §3.2 | Latent, fires on every autoscaler reclaim | (same milestone) | **m82** |
| **P2** | No build queue signal — `bex_build_queue_seconds` does not exist | [code] §3.3 | Every incident in this class presents as silence; **this investigation itself reached a wrong conclusion for lack of it** | ~1h | **m83** |
| **P3** | The BuildKit prewarm DaemonSet does not reach build nodes | [measured] §3.4 | Real but narrow — cold nodes only | ~35m | **m83** |
| **P4** | The cx33 build margin is 305 MiB and unguarded | [measured] §3.5 | None today; **fails silently** if any request grows | ~45m | **m83** |
| **P5** | Operator PUTs an unchanged Deployment every reconcile | [code] §3.6 | Silent perpetual apiserver/etcd churn, per App, forever | ~3h | **m84** |
| **P6** | `bex-tenant-0` is at its max (2/2); serving cannot spill onto build nodes since D8 | [measured] §3.7 | None observed — no pending pods. A headroom risk, not an incident | needs 1 measurement first | **unscoped** |
| **P7** | Push path has no retries; `--dest-tls-verify=false` unconditional | [code] §3.8 | Latent (inert on in-cluster HTTP Zot) | rides M1 | **m82** |
| **P8** | Digest-pinning remainder + no guard | [code] §3.9 | Supply-chain exposure; fifth repeat report | ~2.5h | **m85** |
| **P9** | No build layer cache | [code] §3.10 | Every build recompiles unchanged deps | ~4h | **m86** |
| **P10** | `egress-meter` placement comment premise is false; kpack `ClusterBuilder` staleness unmonitored | [measured]/[code] §3.11 | Hygiene / stale-toolchain drift | sub-hour each | inbox notes |

**The headline:** the investigation started as a capacity question and ended as a **failure-handling** question. There is no capacity shortage (§4). P0–P1 are what is actually costing something.

---

## 3. The defects

### 3.1 (P0) Production is burning guaranteed-to-fail retries, and every failure looks the same — **[measured]**

The six most recent `bex-build` pods on hetzner-prod:

| pod | phase | failing container | exit code |
| --- | --- | --- | --- |
| `bld-beancount-forum-gen-29-97qw4` | Failed | `clone` | **128** |
| `bld-beancount-forum-gen-29-p9p6j` | Failed | `clone` | **128** |
| `bld-…-market-size-gen-1-ggrgl` | Failed | `buildkit` | **1** |
| `bld-…-market-size-gen-1-rl7kl` | Failed | `buildkit` | **1** |
| `bld-…-beancount-cms-v2-gen-203-zpst8` | Succeeded | — | — |
| `bld-…-eden-cms-v2-gen-97-qxnzp` | Running | — | — |

Two distinct failing builds, **two pods each** — `BackoffLimit: 1` (declared `lego/operator/internal/build/build.go:472`, applied at `:649`) spending a second attempt on failures that are deterministic by construction: `git clone` exiting 128 (bad ref / auth / repo) and `buildkit` exiting 1 (the tenant's Dockerfile or build command). **Every one of those retries was guaranteed to fail before it started**, and each held a node-exclusive 7Gi slot for its full duration.

Both also reach the tenant as the same flat `BuildFailed`, so "your git ref is wrong" is indistinguishable from "the platform had a bad day". That ambiguity is also why a build **SLO** is not definable today: there is no `user_failed` vs `infra_failed` split to exclude tenant errors from.

### 3.2 (P1) A disruption mid-build fails the deploy — **[code]**

`build.go:472` sets `backoff := int32(1)`, applied at `build.go:649`. The compensating workaround is the annotation at `build.go:586`, whose own comment records the production symptom: *"a build pod evicted by cluster-autoscaler every few minutes until BackoffLimitExceeded, with no application-code fault."*

So `cluster-autoscaler.kubernetes.io/safe-to-evict: "false"` is load-bearing — and it also prevents legitimate node consolidation. ADR060 D2's `podFailurePolicy` (`Ignore` on `DisruptionTarget`, `FailJob` on tenant exit codes, `backoffLimit: 2` for the unclassified rest) is the designed replacement; `grep` finds no `PodFailurePolicy` anywhere in the operator.

### 3.3 (P2) There is no capacity signal — **[code]**

`w2/m72` shipped `bex_build_outcomes_total{outcome}` and `bex_build_run_seconds`. ADR060 D5's **`bex_build_queue_seconds`** — admission → build pod scheduled, explicitly "the capacity/platform signal" — **does not exist**, nor do `bex_builds_queued` / `bex_builds_active` / `bex_build_push_seconds`.

This is why every incident in this class presented as silence and was diagnosed by hand-descending through logs (2026-08-08 wedged build; 2026-08-11 22-minute Pending, deploy failed by the control-plane gate while the build was healthy). **It is also why the first revision of this document reached a wrong conclusion (§4): with no queue series, capacity had to be argued from arithmetic instead of read from data.**

**Queue time is the platform's SLI; run time is mostly the tenant's.** Conflating them was the 2026-08-11 misattribution.

### 3.4 (P3) The prewarm DaemonSet does not reach build nodes — **[measured]**

`deploy/gitops/base/build-image-prewarm.yaml` sets `nodeSelector: bex.co/pool=tenant` and carries **no `tolerations` block**. D8 registered `bex.co/build-only=true:NoSchedule` on `bex-tenant-lg` / `bex-tenant-burst`; both still carry the `bex.co/pool=tenant` label, so the selector matches but the taint is not tolerated.

Confirmed live: the DaemonSet reports `DESIRED 2 / READY 2`, and both pods sit on `bex-tenant-0-*` — the **serving** nodes. `bex-tenant-lg-4wvlk-s5z8z`, the only node that runs builds, has no prewarm pod.

**Scope correction — narrower than first written.** An earlier revision said "every build cold-pulls BuildKit". Measurement disagrees: the live build's events show `Container image "alpine/git@sha256:…" already present on machine`, because the lg node has been up since 2026-08-16 and kubelet's own image cache does the job. The defect therefore bites **only on cold nodes** — a burst scale-from-zero, or lg being replaced. Still worth the one-line fix (that is exactly when a build is most latency-sensitive), but it is not bleeding on every build.

### 3.5 (P4) The cx33 build margin is 305 MiB and nothing guards it — **[measured]**

See §4 for the full arithmetic. A cx33 build slot **works today** with 305 MiB spare. That margin is thin enough to be fragile: if log-shipper's request grows, another DaemonSet gains a build-taint toleration, or the build request rises, a cx33 stops fitting and **cluster-autoscaler declines to scale up without emitting an error**. A structural guard (hint ≥ build request + DaemonSet floor) is cheap insurance against a failure mode that is invisible by construction.

### 3.6 (P5) The operator PUTs an unchanged Deployment every reconcile — **[code]**

`.pm/w7/028.md`'s chain, confirmed link by link:

- `deployment_projection.go:296` rebuilds `Spec.Template.Spec.Containers` wholesale, setting no `TerminationMessagePath` / `TerminationMessagePolicy`;
- no `SuccessThreshold` on any of the three probes;
- `deployment_projection.go:300` forces `TerminationGracePeriodSeconds`.

The mutated object can never `DeepEqual` the server-defaulted one, so `controllerutil.CreateOrUpdate` returns `Updated` every pass, and `app_controller.go:3398`'s `Owns(&appsv1.Deployment{}, ResourceVersionChangedPredicate{})` feeds the operator's own PUT back into its queue — a self-sustaining reconcile loop per App. Pods never roll (the template hash is computed post-defaulting), so it is pure apiserver/etcd churn.

Scope is wider than one object: **25 `CreateOrUpdate` call sites** — `app_controller.go` (9), `keyvalue_controller.go` (5), `database_controller.go` (5), `build/kpack.go` (3), `build_registry_secret.go` (2), `keyvalue_backup.go` (1) — with three controllers watching owned objects on `ResourceVersionChangedPredicate`. `w9/m57` fixed exactly this mechanism for Services (`Protocol`); nobody generalized it. There is currently **no `OperationResultNone` assertion anywhere** in the operator.

### 3.7 (P6) The serving pool is at its maximum — **[measured]**

| pool | shape | replicas / min / max | taint |
| --- | --- | --- | --- |
| `bex-tenant-0` | cx33 | **2 / 1 / 2 — at max** | none |
| `bex-tenant-lg` | cx43 | 1 / 1 / 1 | `bex.co/build-only` |
| `bex-tenant-burst` | cx33 | **0** / 0 / 2 | `bex.co/build-only` |

`bex-tenant-0` has no headroom, and since D8 tainted the other two pools, serving workloads can no longer spill onto them. **No harm observed** — a cluster-wide pending-pod query returned only the one live build pod, so nothing is currently unschedulable.

This is a headroom risk, not an incident, and it is a **serving** question rather than a build question. Deliberately left unscoped: it needs one measurement (actual vs. requested utilisation on both tenant-0 nodes) before anyone can size it. Recorded here so it is not lost.

### 3.8 (P7) Push path: no retries, TLS verification disabled unconditionally — **[code]**

`build.go:537` — `pushArgs := []string{"copy", "--dest-tls-verify=false"}`, with no `--retry-times`. The TLS half is the already-filed `.pm/w1/046.md` F11 (inert against the in-cluster HTTP Zot, wrong the moment `BEX_REGISTRY` names a TLS registry). ADR060 D4 also wants `zot-config` `gcDelay` pinned above the worst-case push duration and Zot's `scrub` extension enabled — neither is done.

### 3.9 (P8) Digest-pinning remainder — smaller than the register implies — **[code]**

Fifth report of the same deferral (ADR055 F7 → ADR061 #1 → ADR063 #12 → ADR066 #7 → ADR067 #8). Already pinned: `lego/Dockerfile` both FROMs, buildkit / git / skopeo / cosign / busybox-preparer, and **all five native base images** (`native.go:42-46`). Still unpinned:

- `dashboard/Dockerfile:2` and `:45` — both bare `node:22-alpine` (the one user-facing SSR image);
- `keyvalue_controller.go:74` `valkey/valkey:8-alpine`; `:79` `oliver006/redis_exporter:alpine` (**floating — no version at all**);
- `keyvalue_backup.go:53` `alpine:3.21`, plus ADR067 #8's named extension: a runtime `apk add age` in the stage that reads the plaintext RDB;
- `deploy/gitops` — `plugin-barman-cloud:v0.13.0` (version tag, not digest).

Four files plus a guard. The guard is the durable half — without it this becomes a sixth report.

### 3.10 (P9) No build layer cache anywhere — **[code]**

No `--export-cache` / `--import-cache` / `BEX_BUILD_CACHE` in the tree. The Dockerfile and native paths recompile unchanged dependencies on every build; only kpack has any cache.

There is a real design head, not just a flag: BuildKit **deliberately holds no registry credential** — it writes an OCI archive to an emptyDir (`build.go:437`) and skopeo does the authenticated push (`build.go:543`). A registry cache export needs that credential boundary re-settled, which ADR060 D3 explicitly defers to the implementing milestone.

### 3.11 (P10) Hygiene residuals

- **`egress-meter` placement premise is false — [measured].** `lego/operator/config/egress-meter/daemonset.yaml:31` says `# No tolerations: production tenant nodes are the only untainted pool.` D8 falsified that. Measured: the meter (48Mi) runs on both `bex-tenant-0` nodes and **not** on the tainted lg node. Functionally this is probably correct — build nodes host no serving Apps, so there is no tenant App egress to meter — but it is now correct by accident. Confirm that build-time egress is metered as `build_seconds` rather than tenant egress, then either fix the comment or add the toleration deliberately.
- **kpack `ClusterBuilder` staleness unmonitored** (ADR060 D7 residual): no Ready-condition or builder-image-age alerting, so a stale builder silently ships old toolchains and CVEs to every buildpack tenant. Pair with a periodic re-resolve-and-commit cadence for the now-pinned native base digests.

---

## 4. Capacity: question asked, measured, and closed — **do not add machines**

Asked 2026-08-17: *should we add more machines for building?* **Answer: no, and no shape change either.**

### 4.1 What an earlier revision of this document claimed — **[refuted]**

It argued that a cx33 could not host a 7Gi build pod, that `bex-tenant-burst`'s two elastic slots were therefore "ghost capacity", and recommended deleting the burst pool and rotating `bex-tenant-lg` to `min 1 / max 2–3` (cx43). **That reasoning was wrong.** It assumed cilium and the other node DaemonSets carried meaningful memory requests. They do not — they request **0**.

### 4.2 The measurements

| quantity | measured value |
| --- | --- |
| cx33 capacity / **allocatable** memory | 7,937,236Ki (7.57 GiB) / **7,834,836Ki (7.47 GiB)** |
| cx43 capacity / **allocatable** memory | 15,988,560Ki (15.25 GiB) / **15,886,160Ki (15.15 GiB)** |
| build pod **effective** memory request | `max(initContainers)` = **7Gi = 7,340,032Ki** (the `buildkit` initContainer; the `push` container's 128Mi never dominates) |
| DaemonSet memory requests **on a tainted build node** | **log-shipper only: 128Mi + 50Mi = 178Mi** |
| cilium · cilium-envoy · hcloud-csi-node · kube-proxy | **0Mi requested each** |
| egress-meter (48Mi) · build-image-prewarm (8Mi) | absent from build nodes (no toleration) |

- **cx33:** `7,834,836Ki − 7,340,032Ki = 483 MiB` free after the build pod, minus 178Mi log-shipper = **305 MiB spare. It fits.**
- **Scale-from-zero simulation is conservative, not optimistic:** the CA hint (`memory: "8G"` = 7,812,500Ki = 7.451 GiB) is slightly *below* real allocatable, leaving ~283 MiB spare in simulation — the same verdict.
- **cx43:** `15,886,160Ki − 2×7,340,032Ki − 178Mi ≈ 1.0 GiB` spare with two builds. Genuinely comfortable; lg is the right warm tier and stays as-is.

### 4.3 Consequences

- **`.pm/w7/024.md` branch 3 is refuted.** The burst pool is not structurally dead. Branch 4 (the pool was at max during the incident) or an unrelated cause is what remains.
- **There is no observed build capacity shortage.** During measurement `bex-tenant-burst` sat at `replicas 0` with a single active build on lg (memory requests 47% of allocatable) — the warm tier has never been exhausted.
- Options considered and rejected: **(a)** rotate burst to cx43 and **(b)** consolidate into one cx43 pool — both premised on the refuted arithmetic, and both spend peak budget for concurrency nothing has asked for; **(c)** shrink the build request to fit cx33 — rejected on independent merit (heavy builds already OOM against the 7.45 GiB ceiling, and 2 CPU / 8 GB is deliberate parity with Render's Starter pipeline tier); **(d)** delete burst — unnecessary, it works and costs nothing at `replicas 0`.
- Also not proposed: a persistent shared buildkitd pool (ADR060 rejects it — a daemon filesystem becomes a cross-tenant trust boundary) and node-local `hostPath` cache (rejected in ADR034 and again in ADR060).

**What survives from the capacity thread:** only the arithmetic guard (§3.5) — the margin is real but thin, and it breaks silently.

---

## 5. Proposed milestones (in priority order)

Sizes cover implementation tasks only. `/pm` appends the standing closing tasks (Render parity where noted, then Simplify, Test coverage, Closeout) when it materializes.

### m82 — Build failure classification: retry only what retrying can fix (ADR060 D2 + D4) — ~4h · **P0/P1/P7**

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reserved exit codes across clone / native-prepare / buildkit / push / sign + `terminationMessagePolicy: FallbackToLogsOnError` | 45m | — |
| t002 | `podFailurePolicy`: `Ignore` on `DisruptionTarget`, `FailJob` on tenant codes, `backoffLimit: 2` for the rest; retire `safe-to-evict: false`; pre-deploy stays `backoffLimit: 0` | 45m | t001 |
| t003 | Classified outcome `succeeded\|user_failed\|infra_failed\|canceled\|timeout` → App condition reason (splits flat `BuildFailed`) + bex-api deploy record + `bex_build_outcomes_total` classes + `bex_build_retries_total{reason}` | 50m | t002 |
| t004 | D4: skopeo `--retry-times 3`; `--dest-tls-verify` conditioned on registry scheme (closes `w1/046` F11); `zot-config` `gcDelay` ≥ build deadline + `scrub`; `bex_build_push_seconds` / `_errors_total` | 45m | t003 |
| t005 | Infra-success SLO (99.5%) + correlated-failure alert: `infra_failed` across ≥N distinct workspaces in a window | 40m | t004 |

**Definition of done:** a `clone` exit 128 and a `buildkit` exit 1 each fail **once**, attributed to the user, with no second attempt; a build pod killed by node drain retries automatically without consuming the tenant's budget or failing the deploy; OOM never retries; the deploy's failure reason distinguishes the classes across REST/GraphQL/MCP and the dashboard; infra-success rate is a queryable series. Pinned by tests that fail against today's flat classification.

- **Source:** ADR060 D2 + D4 (rollout step 3, "independently shippable"); `.pm/w1/046.md` F11; §3.1, §3.2, §3.8.
- **Goal linkage:** `.pm/GOAL.md` #3 (git push to deploy) — a deploy that fails for a reason the tenant cannot act on is the worst failure mode a PaaS has.
- **Expected outcome:** wasted retries stop; infra-caused failures become invisible to tenants; tenant-caused ones become unambiguous and fast.
- **Why now:** measured on production this same day — two builds each burned a doomed second attempt on a node-exclusive 7Gi slot, and both were indistinguishable from a platform fault. D1 (`w2/m72`) already made build state a first-class status machine; classification is what makes the outcome metric it ships mean anything.
- **Render parity:** expect **include** — the classified reason reaches the deploy record surfaced on REST/GraphQL/MCP + dashboard (`m79`'s `failure_reason` path).

### m83 — Build-plane observability + the two structural guards — ~3h · **P2/P3/P4**

Was "capacity truth + admission honesty"; **all hardware and pool-shape tasks removed** after §4. Retires `.pm/w7/024.md`.

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | `bex_build_queue_seconds` histogram (admission → pod scheduled) + `bex_builds_queued` / `bex_builds_active` gauges | 45m | — |
| t002 | Alert on build queue p95 breach / build `Pending` beyond N minutes, into the existing Prometheus rules | 30m | t001 |
| t003 | Prewarm follows the build pools: add the build-taint toleration + a fail-closed guard that every MachineDeployment carrying the taint has a matching prewarm toleration | 35m | — |
| t004 | `clusterapi-validate.sh` arithmetic guard: every build pool's CA hint ≥ build request + DaemonSet floor, with a red/green self-test | 45m | t003 |
| t005 | D6: global `BEX_MAX_ACTIVE_BUILDS` ceiling + re-count-after-create overshoot correction on the per-workspace admission gate | 45m | t001 |
| t006 | D7: check the build baseline env (`BEX_APP_RECONCILE_WORKERS`, `BEX_MAX_CONCURRENT_BUILDS`, `BEX_MAX_ACTIVE_BUILDS`) into the gitops path | 20m | t005 |

**Definition of done:** queue time and run time are separate series with an alert on the former; the prewarm DaemonSet is scheduled on every node carrying the build taint (its guard turns CI red without the toleration); the arithmetic guard turns CI red if any build pool's hint drops below the build request plus the DaemonSet floor; admission has a global ceiling and self-corrects overshoot; the baseline is readable from git.

- **Source:** ADR060 D5/D6/D7; `.pm/w7/024.md`; §3.3, §3.4, §3.5.
- **Goal linkage:** `.pm/GOAL.md` #2 (basic obs for operation) and #6 (add & remove physical machine).
- **Expected outcome:** the next capacity claim is read from a series instead of argued from arithmetic; cold-node builds stop paying the image-pull tax; a 305 MiB margin can no longer erode silently.
- **Why now:** this investigation is itself the evidence — with no queue series, the capacity question had to be settled by hand against a live cluster, and the first answer was wrong.
- **Render parity:** expect **omit** — operator/infra mechanism, no REST/GraphQL/MCP/UI surface change.

### m84 — Operator reconcile convergence: no `CreateOrUpdate` may PUT on a no-op — ~3h · **P5**

Retires `.pm/w7/028.md`, generalized.

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reproduce: envtest that reconciles twice and asserts every owned object's `resourceVersion` is unchanged — red on Deployment and whatever else it catches | 40m | — |
| t002 | Fix the Deployment projection (match server defaults, or move the seam to `CreateOrPatch`), with no rollout churn on existing production Deployments | 45m | t001 |
| t003 | Sweep the remaining churning sites across App / KeyValue / Database / kpack / build-registry-secret | 50m | t002 |
| t004 | Promote the two-reconcile assertion to a shared invariant applied to every owning controller | 40m | t003 |

**Definition of done:** a second reconcile of an unchanged App / Database / KeyValue issues zero writes — every owned object's `resourceVersion` stable, `CreateOrUpdate` returning `OperationResultNone` — asserted by an invariant covering all three controllers that turns red if a future projection drifts from the server defaults. No pod rollout is triggered on existing production Deployments.

- **Source:** `.pm/w7/028.md` generalized; `w9/m57` is the single-field precedent; §3.6.
- **Goal linkage:** `.pm/GOAL.md` #6 / #2 — perpetual per-App apiserver+etcd write load; etcd write capacity is the scarcest resource on a 3-node control plane.
- **Expected outcome:** operator reconcile load becomes proportional to actual change instead of to App count × time.
- **Why now:** silent, scales with tenants, just discovered with a precise fix sketch — the cheapest it will ever be.
- **Render parity:** expect **omit** — operator-internal, no surface change.

### m85 — Close the digest-pinning inventory + fail-closed CI guard — ~2.5h · **P8**

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin `dashboard/Dockerfile`'s two FROMs to digests | 25m | — |
| t002 | Pin the three KeyValue controller image constants (incl. the floating exporter tag) | 30m | — |
| t003 | Remove the runtime `apk add age` from the plaintext-RDB stage — pin a prebuilt image or bake it (ADR067 #8's named extension) | 45m | t002 |
| t004 | Pin the remaining manifest image refs (barman-cloud plugin and siblings) | 30m | — |
| t005 | Fail-closed CI guard rejecting any unpinned image reference across Dockerfiles / operator constants / gitops manifests, with a red/green self-test — the `w7/m65` 40-hex-actions-guard shape | 45m | t001–t004 |

**Definition of done:** every image reference bex builds from, runs, or ships resolves by digest; the KV backup path installs nothing at runtime in the plaintext stage; the guard turns CI red on a reintroduced floating tag and its own self-test proves it.

- **Source:** ADR067 #8 and its four predecessor reports; §3.9.
- **Goal linkage:** `.pm/GOAL.md` #7 (security review); w7's supply-chain track (`m10`, `m62`, `m65`, `m75`).
- **Expected outcome:** the deferral closes permanently rather than being re-triaged each round; the guard is the durable half.
- **Why now:** deferred five times because it looked like an inventory sweep; it is four files plus a guard.
- **Render parity:** expect **omit**.

### m86 — Per-App registry build cache (ADR060 D3) — ~4h · **P9**

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Settle the credential-isolation shape for cache export (scoped cache-repo credential vs. local export + skopeo copy); record as an ADR060 amendment | 45m | — |
| t002 | BuildKit `--export-cache` / `--import-cache` on `<app-repo>-cache`, `mode=max,image-manifest=true,zstd`, behind `BEX_BUILD_CACHE=registry` | 50m | t001 |
| t003 | Zot per-App ACL + credential coverage for the `-cache` repo (reusing `w7/m36`'s machinery) | 40m | t002 |
| t004 | Retention: exempt `-cache` repos from `mostRecentlyPushedCount`, give them a most-recent-1 + time-bound policy | 40m | t003 |
| t005 | Prove cache loss changes only speed, never results — corrupt/missing cache falls back to a clean build | 35m | t004 |
| t006 | Measure hit-rate + duration delta on a real repeat build, recorded against D5's series | 35m | t005 |

**Definition of done:** a second build of the same repo with unchanged dependencies reuses cached layers and completes measurably faster; a workspace can never read another workspace's cache; a deleted or corrupt cache produces an identical image; image retention can never evict a hot cache and cache growth can never evict a deployable generation; disabling the env gate is byte-identical to today.

- **Source:** ADR060 D3 (rollout step 5); §3.10.
- **Goal linkage:** `.pm/GOAL.md` #3 — build time is the dominant latency in push-to-deploy.
- **Why now — weakest of the set, flagged honestly:** `.pm/FUTURE-MAYBE.md` holds a related-but-distinct entry (build-artifact reuse for `deploy_only`) whose trigger — "build times become a user complaint" — is unconfirmed. **Sequence this after m83:** with capacity now measured as adequate, build *speed* is the remaining lever, and M2's queue/run split is what shows whether pulls or recompiles dominate. Deciding before that data exists would repeat §4.1's mistake.
- **Render parity:** expect **omit** unless the milestone surfaces cache state.

### Inbox notes (sub-hour — NOT milestones)

- **`egress-meter` toleration decision** (§3.11): confirm build-time egress is metered as `build_seconds`, then fix the stale comment or add the toleration deliberately.
- **kpack `ClusterBuilder` staleness monitoring** (§3.11) + a periodic re-resolve-and-commit cadence for the pinned native base digests. ADR060 D7 residual.
- **`bex-tenant-0` serving headroom** (§3.7 / P6): measure actual vs. requested utilisation on both nodes before proposing anything. Explicitly a serving question, not a build question.

---

## 6. Not proposed (checked against `.pm/DO_NOT_DO.md`)

- `.pm/w7/023.md` (LLM token-metering proxy) — its explicit m41 hold lifted with the green 2026-08-18 production proof; ready for a separate scheduling decision, not auto-promoted by this review.
- A static source-IP firewall on build nodes, and vcluster-per-tenant build isolation — both are explicit anti-goals.

## 7. Board bookkeeping when these materialize

- `.pm/w7/024.md` → retire, **noting its branch 3 was refuted by measurement (§4)**, not fixed. Its surviving residue is m83 t004 (the arithmetic guard).
- `.pm/w7/028.md` → retire as superseded by m84.
- `.pm/w1/046.md` F11 → struck by m82 t004 (note it there; the register lives in w1).
