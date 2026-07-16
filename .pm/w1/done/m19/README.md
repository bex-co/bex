# w1 · m19 — Rearchitecture: rebuild the Hetzner substrate right

**Worker:** worker1 **Goal:** Execute docs/rearchitecture.md (absorbed into [docs/ADR002-architecture.md](../../../../docs/ADR002-architecture.md) §The production substrate, 2026-07-11; original in git history) — make declared state and production converge permanently: CAPH-owned private network, three-node tainted control plane, fixed platform and elastic tenant pools, self-managed CAPI, narrow public exposure, and WireGuard-encrypted east-west. **Status:** done 2026-07-15 — **DONE**. The 2026-07-10/11 rebuild and m19.1 pivot restored production under the old quota; the quota residue then landed as 3 CP + 3 platform + an elastic tenant pool, OpenBao 3/3, and four HA platform CNPG clusters. Both acceptance scripts pass on production and two consecutive declarative workflow replays are green.

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on          |
| ---- | --------------------------------------------------------------------------------------------------------- | --- | ------------------- |
| t001 | Backups first: etcd snapshot · OpenBao Raft snapshot · `pg_dump` every CNPG DB → object storage, verified — **DONE** | 45m | —                   |
| t002 | Rewrite the CAPH overlay to the target shape (network · 3×CP · taint · two pools) — **DONE**              | 45m | —                   |
| t003 | GitOps platform scheduling: tolerations + nodeSelector onto `bex-platform` for every platform chart — **DONE** | 60m | —                   |
| t004 | Engine wiring: Cilium WireGuard addon · autoscaler → `incluster-incluster` · CAPI/autoscaler Argo apps — **DONE** | 45m | t002                |
| t005 | Terraform: label-selector node firewall (public ingress = SSH + ICMP only) — **DONE**                                | 30m | —                   |
| t006 | Execute: teardown → rebuild via CI → GitOps converge → restore OpenBao + CNPG data — **DONE**          | 90m | t001,t002,t003,t004,t005 |
| t007 | Pivot: `clusterctl move` to self-managed · in-cluster autoscaler · destroy `bex-infra` node — **DONE** (by m19.1)                | 45m | t006                |
| t008 | External pointers: DNS → new Traefik LB IP · rotate CI kubeconfig secrets — **DONE** (by m19.1)                                  | 30m | t006                |
| t009 | Full acceptance: verify-elastic on self-managed prod · CI green e2e · port scan · close `w1/014` — **DONE** | 45m | t007,t008,t011      |
| t010 | Simplify — `/simplify` over what this milestone changed — **DONE**                                         | 20m | t009                |
| t011 | Test coverage — rewrite verify-elastic and expand verify-substrate — **DONE**                              | 30m | t007,t008           |
| t012 | Closeout — verify DoD holds, then move the milestone to `done/` — **DONE**                                 | 10m | t011                |

## Definition of done

On prod, all observable and re-checkable:

- [x] `app-cluster.yml` runs green end-to-end twice in a row: Actions 29469676741 and 29469734195.
- [x] CAPH owns network `bex` (`10.10.0.0/16`); every machine and both private-IP LB target sets are attached.
- [x] Three Ready+tainted control-plane machines and three Ready+tainted platform machines; OpenBao 3/3 unsealed on distinct nodes; tenant pool autoscaler min 1/max 3.
- [x] KCP `Available=True`, `RollingOut=False`; CAPI objects and autoscaler live in-cluster; no bootstrap server exists.
- [x] All three schedulers carry `scheduler-config.yaml`; `scripts/verify-elastic.sh` passes scale-up, MostAllocated pack, and scale-down.
- [x] No `CSRValidationFailed` in the three controller-manager 24-hour log windows.
- [x] Remote public node scan shows only `:22`; Traefik exposes 22/80/443 and kube-api exposes 443; both LBs use healthy private targets.
- [x] `w1/014` and `w1/m3` are closed under `w1/done/`.

## Source + Goal linkage

- **Source:** docs/rearchitecture.md (diagnosis with live evidence, 2026-07-10; absorbed into ADR002-architecture.md 2026-07-11, original in git history) — promoted from inbox note `w1/014` (KCP blind + CSR loop + immutable `hcloudNetwork` deadlock) and the m7 t005 LB post-mortem; user decision 2026-07-10: no incremental migration, rebuild the architecture right in one shot (downtime accepted).
- **Goal linkage:** V0 roadmap reliability + elasticity (#6); makes m4 sleep + m3 bin-pack economics real on a substrate that can actually scale down/roll machines. Auth/data stay bought (no change to the Ory/OpenBao/CNPG boundary).
- **Render parity closing task: omitted.** Pure infrastructure — no REST/GraphQL/MCP/UI surface change; Render keeps its substrate internal, so does bex. (Indirect parity effect: unblocks `w1/008` autoscaling-config and `w1/013` Postgres HA.)
- **Expected outcome:** a cluster where git → CI → controller is the only change path; every current base-level incident class (CSR rot, KCP blindness, CI red, capacity starvation on the CP) is structurally impossible, not patched.
- **Why now:** the current state is a deadlock — restoring manageability and preserving the production traffic path are mutually exclusive inside the existing architecture (rearchitecture.md §2 symptom D, in git history); every future infra change (k8s upgrades, `w1/008`, `w1/013`, m17 backups) queues behind it.

## Notes

- DO_NOT_DO screen: the t005 firewall restricts **ports** via label selector (auto-inherited by new machines) — not the rejected static source-IP allowlist; nodes expose only key-authenticated `:22` plus ICMP, while LB listeners remain controller-declared.
- Execution ordering inside t006 matters: backups verified **before** teardown; DNS cutover (t008) can start as soon as t006 creates the new Traefik LB.
- k8s stays v1.31 this round (one variable at a time); the version bump is the first beneficiary of a working KCP afterwards.
