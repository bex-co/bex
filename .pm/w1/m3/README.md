# w1 · m3 — Elastic substrate: bin-pack + autoscale

**Worker:** worker1 **Goal:** Make provisioning track aggregate demand (Σ running tiers), not be hand-driven: pack pods onto the fullest node for density, and add/remove machines reactively. This is bex's auto-allocator. **Status:** todo (t001 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Tier → pod requests/limits — **DONE** | — | — |
| t002 | KubeScheduler `MostAllocated` profile (density) | 30m | t001 |
| t003 | Wire cluster-autoscaler (clusterapi provider) | 30m | — |
| t004 | Autoscaler min/max on the `bex-worker-0` MachineDeployment | 20m | t003 |
| t005 | Verify pack + scale-up + scale-down end-to-end | 25m | t002,t004 |

## Definition of done

A pending pod (no room) triggers a new Hetzner node; pods land on the most-allocated node; an emptied node scales down. Machine count tracks Σ demand with no manual `scale`.

## Current state (2026-07-07)

- **t001 done**: tier → pod requests/limits landed in `lego/types/tiers/tiers.yaml` (m8, commit `065b1f7`); the operator translates `spec.tier` → `requests`/`limits` via the shared catalog.
- **t002**: `infra/clusterapi/overlays/hetzner-caph/cluster.yaml` needs a `KubeSchedulerConfiguration` profile dropped into `KubeadmControlPlane.spec.kubeadmConfigSpec.files[]` with `NodeResourcesFit` scoringStrategy `MostAllocated`. No visible effect until multi-node.
- **t003**: `deploy/gitops/base/autoscaler.yaml` is a fully-written placeholder (uncomment the Argo Application). The git remote (`https://kubernetes.github.io/autoscaler`, chart `cluster-autoscaler`, `cloudProvider: clusterapi`) is ready to wire.
- **t004**: `infra/clusterapi/overlays/hetzner-caph/cluster.yaml` `MachineDeployment` needs annotations `cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size` and `-max-size`.
- **t005**: E2E verification requires ≥2 worker nodes (currently `replicas: 0`); spin up 2 and run a pack-then-drain.

## Source

Converted from `.tmp/002-binpack-scheduler.md` and `.tmp/004-cluster-autoscaler.md` (001 already shipped).
