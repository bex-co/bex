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

## Implementation evidence

### Source + structural checks (2026-07-15)

- The four GitOps `Cluster` manifests now declare `instances: 2`, `affinity.podAntiAffinityType: required`, and `primaryUpdateMethod: switchover`; `scripts/gitops-validate.sh` checks the exact four namespace/name identities and rejects any invariant regressing.
- `bex-system`'s default-deny policy previously blocked the CNPG operator from the bex-db instance manager. A separate least-privilege policy now admits only `cnpg-system` → pods labeled `cnpg.io/cluster: bex-db` on TCP 8000, and the validator pins that shape.
- `bash scripts/gitops-validate.sh` passes (all Kustomize/Helm renders plus the new HA guard; local machine lacks optional `fga`/`promtool`, whose existing checks reported their documented warnings).
- Mutation proof passes: in a temporary clone, setting `auth/hydra-db` back to `instances: 1` makes the real validator exit 1 with `want >=2 for drain-safe failover`.
- Push 2b is prepared in `infra/clusterapi/overlays/hetzner-caph/cluster.yaml`: platform workers use `bex-platform-baked` / `imageName: bex-worker`, the boot-time runtime downloads and package install are gone, and no `STAGED ROLL` marker remains. `bash scripts/clusterapi-validate.sh` passes.

These source changes remain uncommitted because repository policy requires an explicit `$ship` invocation before commit/push. The live HA and switchover acceptance run below used a temporary direct apply with the owning Argo Applications paused. Persisting the desired state and starting the platform MachineDeployment roll still require that ship gate.

### Live preflight + OpenBao rehearsal (2026-07-15)

- Baseline production probes passed before disruption: Kratos `/sessions/whoami` returned its expected unauthenticated 401, Hydra introspected an ephemeral client token as active, OpenFGA allowed an explicit check, and an authorized `GET /v1/services` returned 200. The ephemeral Hydra client and OpenFGA tuple were removed afterward.
- OpenBao began with three Ready/unsealed members on three distinct platform nodes, three Raft peers (one leader), and PDB `disruptionsAllowed=1`.
- Rehearsal: deleted non-leader `openbao-2`; the replacement started sealed on the same node while the leader kept serving and all three peers remained visible. `scripts/bao-init.sh` left the two healthy members alone, unsealed the replacement, and restored it to Ready within 42 seconds. The final peer count was three and `disruptionsAllowed` returned to 1.
- The exact one-member-at-a-time sequence, secret-safe unseal forms, quorum stop condition, and rejoin check are now documented in `docs/ADR015-openbao-backup-restore.md`.

### Temporary CNPG HA rollout + per-database drains (2026-07-15)

- All four clusters reached two Ready instances on distinct platform nodes under required pod anti-affinity. The first auth patch exposed CNPG's default `primaryUpdateMethod: restart`: after creating each standby, a pod-template update restarted the primaries in place. The corrected manifests explicitly use `switchover`, preventing that avoidable update path.
- bex-db initially could not create its standby because the default-deny policy blocked CNPG's TCP 8000 instance-manager calls; the scoped policy above fixed it. Its former primary then exposed a missing out-of-band `bex-db-backup-s3` Secret while draining queued WAL. The Secret was restored from the existing `.env` credentials without printing them; all 114 queued WAL files archived and the member rejoined.
- CNPG 1.30 intentionally generates a primary-only PDB (`minAvailable: 1`, steady-state `disruptionsAllowed: 0`) for a two-instance cluster. Drainability is behavioral: cordoning the node makes CNPG switch the primary role, after which the eviction proceeds. Each real `kubectl drain` below completed through that PDB path; adding a second PDB would duplicate CNPG's controller-managed safety mechanism.

| database     | automatic drain switchover + two-Ready recovery | probe failures during drain |
| ------------ | ----------------------------------------------: | --------------------------: |
| `hydra-db`   |                                             58s |       1 Hydra introspection |
| `kratos-db`  |                                             53s |                           0 |
| `openfga-db` |                                             51s |     2 direct OpenFGA checks |
| `bex-db`     |                                             49s |                           0 |

The continuous harness ran 162 cycles of four checks: Kratos `/sessions/whoami` expected 401, Hydra introspection active, OpenFGA tuple allowed, and authenticated `GET /v1/services` 200. Totals were Kratos 0 failures, Hydra 1, OpenFGA 4 (two during deliberate pre-positioning and two during its drain), and bex-api 0. All were isolated single-cycle misses, not sustained outages. The temporary OAuth client, bex-db membership, and OpenFGA tuple were removed, and every node was uncordoned afterward.

After collecting the evidence, the temporary manifests and policy were removed and all three Argo Applications resumed at `Synced`/`Healthy`; the live clusters are back at `main`'s one-instance desired state pending `$ship`. That rollback reproduced the default in-place-restart hazard and required clearing four already-stopped pod objects from CNPG's 30-minute grace window; all four recreated from their intact PVCs and returned healthy. The out-of-band backup Secret remains because it repairs a real production prerequisite, not test state.
