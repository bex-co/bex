# w1 · m3 — Elastic substrate: bin-pack + autoscale

**Worker:** worker1 **Goal:** Make provisioning track aggregate demand (Σ running tiers), not be hand-driven: pack pods onto the fullest node for density, and add/remove machines reactively. This is bex's auto-allocator. **Status:** todo (t001 done, in `done/`)

## Tasks (in order)

| id   | title                                                                         | est | depends_on |
| ---- | ----------------------------------------------------------------------------- | --- | ---------- |
| t001 | Tier → pod requests/limits — **DONE**                                         | —   | —          |
| t002 | KubeScheduler `MostAllocated` profile (density)                               | 30m | t001       |
| t003 | Wire cluster-autoscaler (clusterapi provider)                                 | 30m | —          |
| t004 | Autoscaler min/max on the `bex-worker-0` MachineDeployment                    | 20m | t003       |
| t005 | Verify pack + scale-up + scale-down end-to-end                                | 25m | t002,t004  |
| t006 | Simplify — `/simplify` over what this milestone changed                       | 20m | t005       |
| t007 | Test coverage — script-level checks for the elastic behavior (else close n/a) | 20m | t005       |
| t008 | Closeout — verify DoD holds, then move the milestone to `done/`               | 10m | t007       |

## Definition of done

A pending pod (no room) triggers a new Hetzner node; pods land on the most-allocated node; an emptied node scales down. Machine count tracks Σ demand with no manual `scale`.

## Current state (2026-07-07)

- **t001 done**: tier → pod requests/limits landed in `lego/types/tiers/tiers.yaml` (m8, commit `065b1f7`); the operator translates `spec.tier` → `requests`/`limits` via the shared catalog.
- **t002**: `infra/clusterapi/overlays/hetzner-caph/cluster.yaml` needs a `KubeSchedulerConfiguration` profile dropped into `KubeadmControlPlane.spec.kubeadmConfigSpec.files[]` with `NodeResourcesFit` scoringStrategy `MostAllocated`. No visible effect until multi-node.
- **t003**: `deploy/gitops/base/autoscaler.yaml` is a fully-written placeholder (uncomment the Argo Application). The git remote (`https://kubernetes.github.io/autoscaler`, chart `cluster-autoscaler`, `cloudProvider: clusterapi`) is ready to wire.
- **t004**: `infra/clusterapi/overlays/hetzner-caph/cluster.yaml` `MachineDeployment` needs annotations `cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size` and `-max-size`.
- **t005**: E2E verification requires ≥2 worker nodes (currently `replicas: 0`); spin up 2 and run a pack-then-drain.

## Source + Goal linkage

- **Source:** converted from `.tmp/002-binpack-scheduler.md` and `.tmp/004-cluster-autoscaler.md` (t001 already shipped — see Current state). Retrofitted to current `/pm` conventions 2026-07-08; Closeout (t008) added 2026-07-09.
- **Render parity closing task: omitted.** Pure infra/gitops manifests — no REST/GraphQL/MCP/UI surface change. Render keeps machine provisioning internal (no user-facing API for it); the user-facing Render autoscaling surface (`PUT /services/{id}/autoscaling`) is tracked separately as `w1/008`, gated on this milestone.
- **Goal linkage:** V0 roadmap #6 (add & remove physical machine); the density economics that make tier pricing and free-tier sleep (m4, shipped) pay off.
- **Expected outcome:** node count follows real demand — a pending pod self-provisions a Hetzner machine, an emptied node is removed; no human runs `mock-cluster.sh scale N` in anger.
- **Why now:** a hand-scaled single-node pool is both a capacity ceiling and a cost floor; m4's shipped "sleep = free" only saves money if drained nodes actually scale down. The replica-semantics contract is settled (scale verb, w2/m12; sleep/suspend override the effective count without rewriting `spec.replicas`) — the autoscaler moves **nodes**, never `spec.replicas`.

## Notes

- The work here is gitops/infra yaml (no Go), so the standing closing tasks apply in the m13 style: `/simplify` runs over the changed manifests; test coverage means scripted, asserting verification (extend the t005 script) or an explicit n/a closure — never tautological checks.
