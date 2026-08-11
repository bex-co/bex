# ADR053 — Node instance-type policy (Hetzner cx33 default, cpx32 fallback)

**Status:** Accepted (2026-08-08)

## Context

Every node in the production `hetzner-prod` cluster is a Hetzner Cloud shared-vCPU server provisioned by Cluster API + CAPH from immutable `HCloudMachineTemplate` objects in [`infra/clusterapi/overlays/hetzner-caph/cluster.yaml`](../infra/clusterapi/overlays/hetzner-caph/cluster.yaml). There are four pools: control-plane (3, etcd quorum), platform (min 3 for the anti-affined OpenBao Raft members), tenant-0 (warm baseline, 1), and tenant-burst (cold scale-out, 0→2). A separate sandbox pool is defined in `sandbox-pool.yaml`.

The intended baseline shape is **cx33** — Hetzner's Intel shared-vCPU line, **4 vCPU / 8 GB**. During a window around 2026-07-09 the Intel `cx` line was **create-blocked / out of stock in fsn1**, so every pool was rotated to the AMD **cpx32** (also 4 vCPU / 8 GB, x86_64) as a fallback. The baked worker snapshot is x86, so it boots unchanged on either shape. That fallback is why the live cluster ran entirely on cpx32.

As of **2026-08-08** the Hetzner API reports cx33 (and cx23) **available and available-for-migration in fsn1-dc14** again. The price gap is large:

| Type             | id  | Spec                   | fsn1 gross €/mo |
| ---------------- | --- | ---------------------- | --------------- |
| **cx33**         | 115 | 4 vCPU / 8 GB / 80 GB  | **€8.99**       |
| cx23             | 114 | 2 vCPU / 4 GB / 40 GB  | €6.49           |
| cpx32 (fallback) | 110 | 4 vCPU / 8 GB / 160 GB | €41.99          |

cx33 is the same core/RAM as cpx32 for **~4.7× less** in fsn1. Across ten nodes that is roughly €420/mo → €90/mo.

## Decision

1. **cx33 is the default instance type for every pool** (control-plane, platform, tenant-0, tenant-burst). Same 4 vCPU / 8 GB, same amd64 baked snapshot — no image rebuild, no application changes. **One deliberate exception**: the `bex-tenant-lg` pool is cx43 — see § Large tenant pool below.
2. **cpx32 is the documented fallback**, used only while fsn1 cannot create the `cx` line. The cpx32 templates are retained in the overlay for a fast flip.
3. **The control plane stays on 8 GB cx33, never the 4 GB cx23.** CP live memory is ~3.8 GiB; a 4 GB node has no headroom. cx23 is not used by any pool today.
4. Before switching, confirm fsn1 stock: `GET https://api.hetzner.cloud/v1/datacenters` → `fsn1-dc14.server_types.available_for_migration` must contain the cx33 id (115). During a stock-out, reverse the decision (see Fallback).

## Mechanism — immutable-template rotation

`HCloudMachineTemplate.Spec` is immutable (CAPH v1.1.7 rejects any spec update), so changing a pool's server type is **not** an in-place edit. Each pool gets a **new** template with a distinct name, and the pool's `infrastructureRef` is repointed to it. That repoint is what triggers the rolling node replacement.

- Workers (`bex-tenant-0`, `bex-tenant-burst`, `bex-platform`) each gained a new `…-k134-cx33` template; their MachineDeployments repoint to it.
- The control plane repoints its `KubeadmControlPlane.machineTemplate` to the pre-existing `bex-control-plane` (cx33) template.
- The `cpx32` templates are left in place, unreferenced, ready for a fallback flip.

The guard in [`scripts/clusterapi-validate.sh`](../scripts/clusterapi-validate.sh) enforces the current default: control-plane and burst templates must be `cx33`. On a fallback, flip the guard's expected type to `cpx32` alongside the overlay.

## Rollout order (apply-time, not file-time)

Editing the overlay does not touch the cluster. Applying it does — a rolling replacement of production nodes. Stage the apply; do **not** roll everything at once:

1. **Workers first** (low risk — drain + replace): apply the tenant-burst, tenant-0, and platform template + MachineDeployment changes. CAPI replaces each node; the autoscaler and MachineHealthChecks tolerate the churn.
2. **Control plane last** (sensitive — rolling etcd-member replacement): apply the KCP repoint. CAPI replaces **one** control-plane machine at a time, preserving the 3-member etcd quorum. Watch each member rejoin healthy before the next.

Between steps, verify with `kubectl get machines` / `kubectl top nodes` that the new cx33 nodes are Ready and workloads rescheduled.

## Large tenant pool — `bex-tenant-lg` (cx43, added 2026-08-11)

A fifth pool, **additive**: one always-on cx43 (8 vCPU / 16 GB, €18.49/mo gross fsn1). No existing pool or node was rotated to create it.

**Why a bigger shape rather than more cx33s.** A build Job requests **2 CPU + 7Gi** ([`lego/operator/internal/build/build.go`](../lego/operator/internal/build/build.go), sized node-exclusive on purpose), while a cx33 has ~7.46 GiB allocatable. A build therefore only fits on a node that is _essentially empty_ — adding cx33s does not help once a node carries any tenant workload, because the 7Gi request cannot coexist with it. Raising `bex-tenant-burst`'s max would have bought concurrency but kept every build waiting on a cold scale-from-zero.

16 GB changes the arithmetic: one build leaves ~8 GB for ordinary workloads, or **two builds fit side by side**. `min=max=1` keeps the node warm so a build never waits for provisioning.

**What prompted it (2026-08-11).** A `beancount-cms-v2` build sat `Pending` **22 minutes** — 0/10 nodes could fit it, `bex-tenant-burst` was at `max=2`, and the autoscaler reported `2 max node group size reached`. Two tenants building at once starved each other. The deploy was then closed `build_failed` by bex-api's 35-minute build gate **while the build itself was healthy and still running** — the queueing time counted against the build budget. The build subsequently completed and the operator rolled it out, leaving the deploy record contradicting the live revision. The capacity here removes the trigger; the gate's start-point is a separate defect.

**cx43, not the cx53 originally requested — cx53 is not creatable.** Hetzner's `/v1/datacenters` `server_types.available` list is **unreliable for the whole cx line**: on 2026-08-11 it reported cx33, cx43 _and_ cx53 unavailable in all six datacenters, while cx33 nodes were demonstrably being created that same hour. Each type was therefore probed empirically (create → inspect → delete):

| type | spec | fsn1 €/mo | probe result |
| --- | --- | --- | --- |
| cx53 | 16 vCPU / 32 GB | 34.99 | ❌ `resource_unavailable — error during placement` |
| cx43 | 8 vCPU / 16 GB | 18.49 | ✅ created |
| cx33 | 4 vCPU / 8 GB | 8.99 | ✅ created |
| cpx42 | 8 vCPU / 16 GB | 81.00 | ✅ created (AMD fallback, 4.4× cx43) |

**Do not trust the `available` list for the cx line — probe before concluding.** If cx53 returns, moving this pool to it follows the immutable-template rotation above (new `…-cx53` template + `infrastructureRef` repoint). `cpx42` is the fallback if cx43 stocks out, mirroring the cx→cpx path already documented below.

The guard in `clusterapi-validate.sh` pins control-plane and tenant-burst to cx33 and deliberately does **not** constrain this pool's type, so the exception cannot silently spread to the pools whose cost profile the guard exists to protect.

## Fallback (fsn1 cx stock-out)

If cx33 becomes uncreatable in fsn1 again:

1. Repoint each worker MachineDeployment back to its `…-k134-cpx32` template and the KCP back to `bex-control-plane-cpx32`.
2. Flip the two `clusterapi-validate.sh` guards back to expect `cpx32`.
3. Apply, workers first then control plane, as above.

This is exactly the rotation performed in the 2026-07 outage, in reverse.

## Consequences

- ~4.7× lower per-node compute cost with zero capability change (same specs, same arch, same image).
- One extra immutable template per pool lives in the overlay (the cpx32 fallback and the cx33 default coexist); stale unreferenced templates are pruned by hand per the roll runbook, as with prior rotations.
- The sandbox pool (`sandbox-pool.yaml`) is out of scope here and remains cpx32; it can be rotated to cx33 with the same pattern when convenient, or left to scale-to-zero.
