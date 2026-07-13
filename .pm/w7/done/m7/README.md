# w7 · m7 — Least-privilege platform RBAC (operator + bex-api secret scoping)

**Worker:** worker7 **Goal:** The operator and bex-api hold only the Kubernetes permissions they actually use — no cluster-wide unrestricted Secret read — so a compromised platform pod can no longer read every credential in `bex-system`/`secrets`/`monitoring`, with a CI guard that fails on any reintroduced over-broad grant. **Status:** done

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Scope the operator's `secrets` access — namespace-scoped Role(s) / narrowed verbs instead of a cluster-wide grant | 45m | —          | — **DONE** |
| t002 | Audit + scope the bex-api ServiceAccount RBAC; narrow any over-broad grants                                | 45m | —          | — **DONE** |
| t003 | CI guard that fails on any reintroduced cluster-wide unrestricted secrets read                             | 30m | t001, t002 | — **DONE** |
| t004 | Simplify — `/simplify` over the code/manifests this milestone changed                                     | 20m | t003       | — **DONE** |
| t005 | Test coverage — meaningful tests for the scoped RBAC + CI guard                                            | 30m | t003       | — **DONE** |
| t006 | Closeout — DoD verified, milestone moved to `done/`                                                        | 15m | t005       | — **DONE** |

## Definition of done

Neither the operator nor bex-api can read Secrets cluster-wide: the operator's `secrets` grant is scoped to the namespaces it actually writes/reads (the apps namespace + the build namespace, or narrowed by verb/`resourceNames`), and bex-api's ServiceAccount holds only the grants it uses; a regression that re-broadens either grant to a cluster-wide unrestricted secrets read fails a CI check (`scripts/gitops-validate.sh` or an RBAC assertion test) before it can merge/apply. The operator and bex-api still function (build clone-secret reads, tenant Secret creation, OpenBao k8s-auth, App-CR reconcile all unaffected).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w7` (2026-07-12). Verified 2026-07-12: `lego/operator/config/rbac/role.yaml:6-19` grants `secrets` `get/list/watch/create/delete/patch/update` **cluster-wide** via a ClusterRole (no namespace scope, no `resourceNames`), broader than the operator's actual use (clone Secrets in the App + build namespaces, tenant Secrets it creates).
- **Goal linkage:** GOAL.md V0 #7 (security review); defense-in-depth blast-radius reduction complementing the w7 network/workload/API hardening already shipped (m1–m5).
- **Expected outcome:** operator-pod (or bex-api-pod) compromise no longer yields cluster-wide secret read — database credentials, API keys, and TLS keys in `bex-system`/`secrets`/`monitoring` are out of reach unless the pod's scoped grant covers them; a CI guard prevents silent regression.
- **Why now:** cheap, high-leverage least-privilege tightening that reduces the blast radius of the very compromise scenarios m1–m4 assume; best done before real tenants and before the RBAC surface grows further.
- **Render parity: omitted** — pure Kubernetes RBAC / platform-infra change with no REST/GraphQL/MCP/UI surface. Closing tasks are Simplify → Test coverage → Closeout only.
