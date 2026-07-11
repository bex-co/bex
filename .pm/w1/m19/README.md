# w1 · m19 — Rearchitecture: rebuild the Hetzner substrate right

**Worker:** worker1 **Goal:** Execute [docs/rearchitecture.md](../../../docs/rearchitecture.md) — one planned rebuild that makes the declared state and reality converge permanently: CAPH owns the private network, 3-node tainted control plane, platform/tenant worker pools, self-managed CAPI (pivot, no pet mgmt node), public exposure reduced to 80/443/6443/22 + WireGuard-encrypted east-west. **Status:** dormant pending quota (see `.pm/FUTURE-MAYBE.md`) — t001–t005 + t007 + t008 done (t005 closed 2026-07-11: firewall applied by infra.yml, port scan :22-only verified). **m19.1 executed 2026-07-11, prod fully serving**: pivot done (self-managed, `bex-infra` destroyed), DNS both zones → `49.12.20.236`, platform + all 3 Apps public with LE certs, both workflows green on the SSH-to-CP path. Remaining here (t006 residue + t009–t012) needs the Hetzner quota raise (user parked it in FUTURE-MAYBE 2026-07-11): revert KCP→3 + platform max→3, OpenBao 3/3 + first `bao-init.sh`, then full acceptance. Interim shape: 1 CP + 2 platform + 1 tenant (KCP webhook forbids 2 — even counts illegal with stacked etcd). Six root-caused fixes shipped en route (see t006 op log). t003 finished across two sessions (sweep agent killed mid-run 14:18, resumed + completed same day): full audit in done/t003.md — platform pool for every chart, CP exemptions recorded for hccm/cilium-operator/etcd-backup (bootstrap-critical), Argo CD patched post-install in deploy.yml, autoscaler scheduling on the Argo Application only (mgmt cluster has no bex.co labels). t004 deviation: CAPI providers stay clusterctl-owned (recorded in cluster-api.yaml), autoscaler is the one new Argo app. (t002: overlay rewritten — hcloudNetwork 10.10.0.0/16 CAPH-owned, KCP 3×tainted, bex-platform pool min2/max3 + tenant pool, scheduler config intact, 15 docs validated; all dumps + etcd snapshot in S3; OpenBao n/a: prod never initialized — env-vars never worked on prod, fresh `bao-init.sh` moves into t006; etcd-backup chain had been broken ≥3 days, fixed live). Discoveries logged in t001; symptom A (CP kubelet cert) has now fully materialized — exec/logs to CP pods dead.

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on          |
| ---- | --------------------------------------------------------------------------------------------------------- | --- | ------------------- |
| t001 | Backups first: etcd snapshot · OpenBao Raft snapshot · `pg_dump` every CNPG DB → object storage, verified — **DONE** | 45m | —                   |
| t002 | Rewrite the CAPH overlay to the target shape (network · 3×CP · taint · two pools) — **DONE**              | 45m | —                   |
| t003 | GitOps platform scheduling: tolerations + nodeSelector onto `bex-platform` for every platform chart — **DONE** | 60m | —                   |
| t004 | Engine wiring: Cilium WireGuard addon · autoscaler → `incluster-incluster` · CAPI/autoscaler Argo apps — **DONE** | 45m | t002                |
| t005 | Terraform: label-selector port firewall (public ingress = 80/443/6443/22 only) — **DONE**                             | 30m | —                   |
| t006 | Execute: teardown → rebuild via CI → GitOps converge → restore OpenBao + CNPG data                         | 90m | t001,t002,t003,t004,t005 |
| t007 | Pivot: `clusterctl move` to self-managed · in-cluster autoscaler · destroy `bex-infra` node — **DONE** (by m19.1)                | 45m | t006                |
| t008 | External pointers: DNS → new Traefik LB IP · rotate CI kubeconfig secrets — **DONE** (by m19.1)                                  | 30m | t006                |
| t009 | Full acceptance: verify-elastic on self-managed prod · CI green e2e · port scan · close `w1/014`           | 45m | t007,t008           |
| t010 | Simplify — `/simplify` over what this milestone changed                                                    | 20m | t009                |
| t011 | Test coverage — scripted, asserting rearch verification (extend verify-elastic / add verify-substrate)     | 30m | t009                |
| t012 | Closeout — verify DoD holds, then move the milestone to `done/`                                            | 10m | t011                |

## Definition of done

On prod, all observable and re-checkable:

- `app-cluster.yml` runs **green end-to-end** on main (apply → wait → autoscaler → addons), twice in a row (idempotence).
- `HetznerCluster.spec.hcloudNetwork.enabled: true` — CAPH owns network `bex` (`10.10.0.0/16`); every machine (CP + workers) attached at creation; **zero out-of-band network state**.
- 3 control-plane machines Ready and **tainted** (no platform workloads on CP); `bex-platform` pool ≥2 runs the platform (OpenBao **3/3 Ready** — the 28h-Pending regression class is dead); tenant pool carries the m3 autoscaler annotations (min 0).
- KCP `Available=True`, no stuck `RollingOut`; CAPI objects live **in the cluster itself** (self-managed; `bex-infra` server destroyed, Terraform definition retained).
- kube-scheduler runs `--config=/etc/kubernetes/scheduler-config.yaml` (closes the last m3/t008 leg); `scripts/verify-elastic.sh` passes all three phases against the self-managed pair.
- No `CSRValidationFailed` events over a 24h window (serving-cert rotation healthy at `cluster-signing-duration: 6h`).
- Both LBs use private-net targets; a public port scan of node IPs shows **only :22**; LB fronts expose only 80/443 (Traefik) and 6443 (kube-api).
- `w1/014` closed (root cause removed); `w1/m3` closes via its own t008.

## Source + Goal linkage

- **Source:** [docs/rearchitecture.md](../../../docs/rearchitecture.md) (diagnosis with live evidence, 2026-07-10) — promoted from inbox note `w1/014` (KCP blind + CSR loop + immutable `hcloudNetwork` deadlock) and the m7 t005 LB post-mortem; user decision 2026-07-10: no incremental migration, rebuild the architecture right in one shot (downtime accepted).
- **Goal linkage:** V0 roadmap reliability + elasticity (#6); makes m4 sleep + m3 bin-pack economics real on a substrate that can actually scale down/roll machines. Auth/data stay bought (no change to the Ory/OpenBao/CNPG boundary).
- **Render parity closing task: omitted.** Pure infrastructure — no REST/GraphQL/MCP/UI surface change; Render keeps its substrate internal, so does bex. (Indirect parity effect: unblocks `w1/008` autoscaling-config and `w1/013` Postgres HA.)
- **Expected outcome:** a cluster where git → CI → controller is the only change path; every current base-level incident class (CSR rot, KCP blindness, CI red, capacity starvation on the CP) is structurally impossible, not patched.
- **Why now:** the current state is a deadlock — restoring manageability and preserving the production traffic path are mutually exclusive inside the existing architecture (rearchitecture.md §2 symptom D); every future infra change (k8s upgrades, `w1/008`, `w1/013`, m17 backups) queues behind it.

## Notes

- DO_NOT_DO screen: the t005 firewall restricts **ports** via label selector (auto-inherited by new machines) — not the rejected static source-IP allowlist; `:22`/`:6443` stay authentication-only per the recorded baseline.
- Execution ordering inside t006 matters: backups verified **before** teardown; DNS cutover (t008) can start as soon as t006 creates the new Traefik LB.
- k8s stays v1.31 this round (one variable at a time); the version bump is the first beneficiary of a working KCP afterwards.
