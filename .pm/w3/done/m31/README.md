# w3 · m31 — Tenant namespace isolation: workspace = namespace

**Worker:** worker3 **Goal:** migrate bex's tenant boundary from a shared `default` namespace + label-scoped policies (ADR022 Option B) to per-tenant namespaces — `<ws>` (hosting) + `<ws>-sandbox` (regime split) — with zero-trust east-west and per-namespace `ResourceQuota`, preserving scale-to-zero. **Status:** **DONE** 2026-07-29 — all tasks complete + validated on prod: per-tenant `<ws>` (+`<ws>-sandbox`) namespaces, plan-tier ResourceQuota replacing `BEX_MAX_*`, per-namespace RBAC, no-downtime migration (18/18), namespace-scoped NetworkPolicies + operator per-App-policy gating + cluster-wide node/metadata egressDeny; reachability matrix 5/5, bin-pack, and scale-to-zero re-asserted per-namespace. ADR022 Option B superseded; ADR043 implemented.

## Tasks (in order)

| id   | title                                                                                                                            | est | depends_on          |
| ---- | ------------------------------------------------------------------------------------------------------------------------------- | --- | ------------------- |
| t001 | Namespace lifecycle: workspace create/delete → `<ws>` (+`<ws>-sandbox`) namespace + base `ResourceQuota`/`LimitRange`/NetworkPolicy | 2h  | —                   | — **DONE** |
| t002 | Operator + bex-api + egress-meter: multi-namespace tenant scope (Secret cache, projector placement); place App/Database/KeyValue workloads in `<ws>` | 3h  | t001                | — **DONE** |
| t003 | RBAC: operator + bex-api + ssh-gateway authority over all tenant namespaces (per-`<ws>` RoleBindings stamped by t001's reconciler; no cluster-wide Secrets grant) | 2h  | t002                | — **DONE** |
| t004 | Per-tenant `ResourceQuota`/`LimitRange` replace `BEX_MAX_*` app-code caps; retire shared `tenant-quotas.yaml`                   | 2h  | t001, t002          | — **DONE** |
| t005 | NetworkPolicy re-scope: namespace default-deny + same-workspace + platform allows; promote the node/metadata Cilium egressDeny to clusterwide (it is namespaced-to-`default` today); retain platform-lockdown/registry-ACL/hardening | 3h  | t002                | — **DONE** |
| t006 | Migration: move existing `default` workloads to per-tenant namespaces, no downtime                                              | 2h  | t002, t003, t004, t005 | — **DONE** |
| t007 | Verify scale-0 + bin-pack on the shared tenant pool; re-assert the tenant-isolation reachability matrix per-namespace           | 2h  | t006                | — **DONE** |
| t008 | Simplify (`/simplify` over changed code)                                                                                        | 30m | t007                | — **DONE** |
| t009 | Test coverage (namespace lifecycle, per-tenant quota, NetworkPolicy, scale-0)                                                   | 2h  | t007                | — **DONE** |
| t010 | Closeout                                                                                                                        | 15m | t009                | — **DONE** |

## Definition of done

Every workspace's App/Database/KeyValue workloads run in their own `<ws>` namespace (and each workspace has a `<ws>-sandbox` ready for m32); per-tenant `ResourceQuota` enforces plan tiers at the API server (no `BEX_MAX_*` in the hot path); cross-tenant traffic is denied by namespace-scoped default-deny; `scripts/verify-tenant-isolation.sh` passes per-namespace; `scripts/verify-elastic.sh` proves scale-to-zero + bin-pack are unchanged on the shared tenant node pool; ADR022's label-scoped per-App policies are removed and [ADR043](../../docs/ADR043-tenant-namespace-isolation.md) is implemented.

## Source + Goal linkage

- **Source:** [docs/ADR043-tenant-namespace-isolation.md](../../docs/ADR043-tenant-namespace-isolation.md) (supersedes [ADR022](../../docs/ADR022-tenant-isolation.md) Option B).
- **Goal linkage:** platform/tenancy integrity — structural tenant boundary, per-tenant quota fairness, entity alignment (workspace = namespace). Enables pillar 5's per-tenant `<ws>-sandbox` (m32).
- **Expected outcome:** tenant isolation is structural (namespace), not conventional (label); one tenant can no longer starve the shared quota or cross another tenant's boundary through a misconfigured policy.
- **Why now:** m32 (sandbox substrate) needs per-tenant `<ws>-sandbox`; doing the hosting namespace migration first yields one consistent tenancy model instead of a hosting/sandbox split. Also a GA-hardening item.
- **Render parity:** OMITTED — pure infra/tenancy refactor with no tenant-facing REST/GraphQL/MCP/UI surface change (tenants never see namespaces; the API is unchanged). Recorded here per the `/pm` convention.
- **Cross-theme note:** this is platform/tenancy work, not observability; placed in w3 by explicit user direction (2026-07-27), recorded in `.pm/DO_NOT_DO.md` #18.
