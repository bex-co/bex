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

## Source

Converted from `.tmp/002-binpack-scheduler.md` and `.tmp/004-cluster-autoscaler.md` (001 already shipped).
