# w1 · m38 — Platform-pool drainability: CNPG HA + the staged baked-image roll

**Worker:** worker1 **Goal:** Any platform node can be drained without an auth/control-plane outage: the four platform CNPG databases become HA with anti-affinity, the OpenBao unseal-during-roll runbook is rehearsed, and the deferred Push 2b (platform pool → baked `bex-worker` image) lands safely. **Status:** done

## Tasks (in order)

| id   | title                                                                                                        | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | CNPG HA: `instances: 2`+ with pod anti-affinity for `hydra-db`/`kratos-db`/`openfga-db`/`bex-db` in gitops; replicas verified on distinct platform nodes — **DONE** | 60m | —          |
| t002 | Prove switchover: per-DB primary disruption with auth (Kratos/Hydra/OpenFGA) and bex-api staying up — **DONE** | 45m | t001       |
| t003 | OpenBao roll runbook: one-node-at-a-time unseal sequence documented + rehearsed — **DONE**                     | 30m | —          |
| t004 | Execute Push 2b: platform pool → `bex-worker` baked image per the `STAGED ROLL` markers, one node at a time — **DONE** | 45m | t002, t003 |
| t005 | Simplify — `/simplify` over the code this milestone changed — **DONE**                                         | 20m | t004       |
| t006 | Test coverage — gitops structural guard: platform DBs must stay ≥2 instances with anti-affinity — **DONE**      | 30m | t004       |
| t007 | Closeout — DoD met → move milestone to `done/` — **DONE**                                                      | 10m | t006       |

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
- Push 2b is encoded in `infra/clusterapi/overlays/hetzner-caph/cluster.yaml`: platform workers use `bex-platform-baked` / `imageName: bex-worker`, the boot-time runtime downloads and package install are gone, and no `STAGED ROLL` marker remains. `bash scripts/clusterapi-validate.sh` passes.

The durable source landed in `314bd4ef` (`feat(infra): make platform pool drain-safe`). Follow-up acceptance findings landed in `5956eea2` (pin the platform autoscaler floor at three) and `4fc532db` (two node-separated replicas + PDBs for the production Traefik/Hydra/Kratos/OpenFGA/bex-api request path). Argo converged the live cluster to those commits and the deploy workflow's later image-pin commits without manual desired-state drift.

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

After collecting the pre-ship evidence, the temporary manifests and policy were removed and all three Argo Applications resumed at `Synced`/`Healthy`. That rollback reproduced the default in-place-restart hazard and required clearing four already-stopped pod objects from CNPG's 30-minute grace window; all four recreated from their intact PVCs and returned healthy. The out-of-band backup Secret remains because it repairs a real production prerequisite, not test state. Shipping `314bd4ef` then made the same HA shape durable.

### Baked platform roll, findings, and corrections (2026-07-15)

- The four durable HA clusters reached 2/2 Ready by `22:20:52Z`. The first baked Machine was Ready before the old pool rolled; every replacement's `HCloudMachine.spec.imageName` was `bex-worker`.
- The live roll exposed two controller interactions that the source-only preflight could not: CAPI selected two old Machines in one rollout reconciliation, and a later unpause raced the autoscaler floor of two, which changed the MachineDeployment from three to two and selected one baked Machine alongside the last old Machine. OpenBao's PDB serialized member eviction and CNPG kept data safe, but two nodes were cordoned concurrently. The first port-forward-only harness lost its tunnels when their selected pods moved, so its resulting transport-failure counts were discarded rather than reported as service outages.
- The two-node event produced a real public-edge gap while the singleton Traefik pod cold-pulled for about 90 seconds. That finding expanded the fix: the platform autoscaler is now fixed at `min=3`/`max=3` (three required-anti-affinity OpenBao members need three schedulable nodes), and the synchronous production request path now runs two required-anti-affinity replicas with `minAvailable: 1` PDBs. The local one-worker CAPD overlay keeps its single-replica defaults.
- OpenBao was unsealed after every reschedule, always with two healthy peers first: `openbao-1` at `22:26:20Z–22:26:33Z` during the first replacement; again at `22:42:07Z–22:42:22Z` after the autoscaler race; and `openbao-2` at `22:48:02Z–22:48:21Z` after the corrected third baked node joined. Final state was three unsealed members, three Raft peers, and `disruptionsAllowed=1`.
- Final MachineDeployment state: three Ready/Available Machines, all `bex-platform-7sh4l-*`, all `imageName: bex-worker`, autoscaler floor/ceiling `3/3`, `Available=True`, `RollingOut=False`, and no pause annotation. The original `bex-platform-b5tch-*` Machines and all `STAGED ROLL` markers are gone.

### Final sequential single-node acceptance (2026-07-15)

After the corrections were live, two ordinary `kubectl drain` runs exercised one node at a time with all nodes restored between runs:

| node | boundary exercised | eviction window | full 2/2 DB + 3/3 OpenBao recovery | probe result |
| --- | --- | ---: | ---: | --- |
| `bex-platform-7sh4l-xjggj` | one replica of all five request-path Deployments + four DB standbys + `openbao-1` | 10s (`23:00:04Z–23:00:14Z`) | `23:01:56Z`; unseal `23:01:21Z–23:01:39Z` | one isolated Kratos and Hydra transition miss; OpenFGA and bex-api zero |
| `bex-platform-7sh4l-kprwk` | all four DB primaries + one replica of all five request-path Deployments + `openbao-0` | 31s (`23:06:41Z–23:07:12Z`) | `23:08:54Z`; unseal `23:08:07Z–23:08:25Z` | LB-pinned harness: Kratos 3, Hydra 1, OpenFGA 0, bex-api 2 isolated misses over 123 drain/recovery cycles; no sustained failure, then 50 consecutive clean cycles |

The first public harness also revealed intermittent Cloudflare-origin timeouts after the cluster was healthy. Direct TLS probes pinned to the real Hetzner LoadBalancer were clean across both Traefik pods and both Kratos pods, so the authoritative primary-drain harness used `curl --resolve` to retain the real LB/TLS/Traefik path while excluding Cloudflare as an unrelated variable. The Hetzner API reported every LB target healthy.

Final live audit: four CNPG Clusters healthy at 2/2 with each primary/replica on distinct nodes; Hydra, Kratos, OpenFGA, Traefik, and bex-api each 2/2 Ready on distinct nodes with PDB allowance 1; OpenBao 3/3 unsealed with three peers; every platform node Ready and schedulable; owning Argo Applications `Synced`/`Healthy`. Temporary Hydra clients, tenant memberships, and OpenFGA tuples were verified at zero.

### Simplify + verification

- The simplification pass kept the HA policy production-only where local CAPD has one worker, used each chart's native replica/affinity/PDB values, and narrowed the CNPG NetworkPolicy to only `cnpg-system` → `bex-db` TCP 8000. No extra controller or bespoke rollout script remains.
- `bash scripts/gitops-validate.sh`, `bash scripts/clusterapi-validate.sh`, `git diff --check`, shell syntax checks, and `make test` from `lego/operator/` pass. Mutation tests prove `instances: 1`, platform autoscaler `min=2`, and a request-path `replicaCount: 1` each fail their owning structural guard.
- GitHub Actions passed GitOps render, Cluster API validation, operator/backend/dashboard suites, backend lint, docs formatting, and secret scan for the shipped commits; the production app-cluster workflow also completed successfully after the platform-floor correction.
