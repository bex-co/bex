# w7 · m1 — East-west tenant isolation: default-deny network for tenant workloads

**Worker:** worker7 **Goal:** Tenant pods can no longer reach other tenants' pods/datastores or bex's platform services over the flat pod network — cross-workspace traffic is denied by default while Traefik ingress, same-workspace private services, own datastores, and public-internet egress keep working. **Status:** done

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | ADR `docs/ADR022-tenant-isolation.md`: threat model + namespace-tier mechanism choice                             | 45m | —          | — **DONE** |
| t002 | Workspace identity labels: projector stamps App CRs, operator propagates to pods                           | 45m | t001       | — **DONE** |
| t003 | Tenant NetworkPolicies: default-deny + Traefik / same-workspace / own-datastore / internet allows          | 60m | t002       | — **DONE** |
| t004 | Platform-side lockdown: deny apps+build namespaces at bex-system · bex-registry · OpenBao · monitoring     | 45m | t001       | — **DONE** |
| t005 | Verification script: cross-tenant + platform reachability matrix                                           | 45m | t003, t004 | — **DONE** |
| t006 | Simplify — `/simplify` over the code this milestone changed                                                | 20m | t005       | — **DONE** |
| t007 | Test coverage — meaningful tests for label propagation + policy generation                                 | 30m | t005       | — **DONE** |
| t008 | Closeout — DoD verified, milestone moved to `done/`                                                        | 15m | t007       | — **DONE** |

## Definition of done

On a cluster with apps in two distinct workspaces: from a tenant pod, connections to another workspace's pod, its CNPG Postgres and Valkey services, bex-api's internal API (`:8091`), and OpenBao (`:8200`) are all **blocked**; the same pod still reaches its own workspace's private services and datastores, still serves via its Traefik URL, and still has public-internet egress. The t005 script proves the whole matrix and exits 0.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w7` (2026-07-09, take 2). Verified 2026-07-09: all tenant Apps project into one namespace (`BEX_CP_APPS_NAMESPACE`, default `default` — `lego/backend/cmd/api/main.go:137`); zero tenant NetworkPolicies in-tree (only the scaffolded `lego/operator/config/network-policy/allow-metrics-traffic.yaml`), so the pod network is flat.
- **Goal linkage:** GOAL.md V0 #7 (security review) and #5 (multi-tenant); the w1/m6 removal note's anticipated namespace-tier re-scope (`DO_NOT_DO.md` ladder: namespace → microVM, never vcluster).
- **Expected outcome:** tenant↔tenant lateral movement and tenant→platform access are closed at the CNI layer, before real tenants exist to migrate.
- **Why now:** w1/m9 closes the API front door (enforced OpenFGA); the moment two tenants coexist, the flat network is the open side door — and policies are cheapest to ship while there are no tenant workloads to grandfather.
- **Render parity: omitted** — pure mechanism (CNI policies + operator/projector labels); no REST/GraphQL/MCP/UI surface change.
