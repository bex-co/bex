# w1 · m38 — Platform-pool drainability: CNPG HA + the staged baked-image roll

**Worker:** worker1 **Goal:** Any platform node can be drained without an auth/control-plane outage: the four platform CNPG databases become HA with anti-affinity, the OpenBao unseal-during-roll runbook is rehearsed, and the deferred Push 2b (platform pool → baked `bex-worker` image) lands safely. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                        | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | CNPG HA: `instances: 2`+ with pod anti-affinity for `hydra-db`/`kratos-db`/`openfga-db`/`bex-db` in gitops; replicas verified on distinct platform nodes | 60m | —          |
| t002 | Prove switchover: per-DB primary disruption with auth (Kratos/Hydra/OpenFGA) and bex-api staying up            | 45m | t001       |
| t003 | OpenBao roll runbook: one-node-at-a-time unseal sequence documented + rehearsed                                | 30m | —          |
| t004 | Execute Push 2b: platform pool → `bex-worker` baked image per the `STAGED ROLL` markers, one node at a time    | 45m | t002, t003 |
| t005 | Simplify — `/simplify` over the code this milestone changed                                                    | 20m | t004       |
| t006 | Test coverage — gitops structural guard: platform DBs must stay ≥2 instances with anti-affinity                 | 30m | t004       |
| t007 | Closeout — DoD met → move milestone to `done/`                                                                 | 10m | t006       |

## Definition of done

Draining any single platform node neither denies PDB evictions nor drops auth/control-plane availability (proven by the t002 disruption runs and the t004 roll itself); all platform-pool nodes run the baked `bex-worker` image and the `STAGED ROLL` markers in `infra/clusterapi/overlays/hetzner-caph/cluster.yaml` are gone; a structural guard fails any future single-instance regression of the four platform DBs.

## Source + Goal linkage

- **Source:** promotes `w1/022` — renumbered `w1/done/023.md` at promotion, its original number had collided with the already-done REST-error-body note — (filed 2026-07-15 by `w1/m36`'s platform-roll pre-flight: all four CNPG clusters single-instance and co-located on one node, each with an allowed-disruptions=0 PDB — the pool is un-drainable; OpenBao members come back sealed on reschedule). The note itself marks prerequisite #1 "worth its own milestone".
- **Goal linkage:** platform reliability (GOAL.md de-risking; the m19.1 review's standing `bex-db` single-copy risk) and finishing `w1/m36`'s deferred Push 2b.
- **Expected outcome:** node maintenance, k8s upgrades, and future rolls stop being auth outages; a single platform-node failure no longer takes Kratos+Hydra+OpenFGA+bex-db down together.
- **Why now:** m36 shipped its tenant half and left Push 2b explicitly gated on this; every day un-drained is a day one node holds the whole auth plane.
- **Render parity closing task: omitted** — pure platform infra; no REST/GraphQL/MCP/UI surface change.
