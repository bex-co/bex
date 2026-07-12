# w6 · m8 — Scope down bex-api's cluster-wide RBAC

**Worker:** worker6 **Goal:** the bex-api ServiceAccount can no longer read Secrets/pods/logs outside the namespaces it actually serves. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                       | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design the split: keep `app.bex.co` verbs cluster-wide, move `secrets`/`pods`/`pods/log` to a namespace-scoped Role for `BEX_API_NAMESPACE`/`BEX_CP_APPS_NAMESPACE` | 45m | —          |
| t002 | Implement: split `lego/operator/config/api/rbac.yaml` into a ClusterRole (apps-only) + Role/RoleBinding(s) for secrets/pods/pods-log | 1h  | t001       |
| t003 | Verify logs API, env-vars API, and managed-DB Secret reads still work against a live/mock cluster after the split           | 45m | t002       |
| t004 | Document the operator manager-role's cluster-wide `secrets` CRUD as a deliberate, justified exception, or file a defense-in-depth follow-up | 20m | t002       |
| t005 | Simplify — `/simplify` over the RBAC manifests + any Go changed                                                              | 15m | t003,t004  |
| t006 | Test coverage — envtest/integration asserting the bex-api SA cannot read Secrets in `auth`/`secrets`/`bex-system`/`argocd`   | 30m | t003       |
| t007 | Closeout — verify DoD met, then move the milestone to `done/`                                                                | 10m | t005,t006  |

## Definition of done

The bex-api ServiceAccount can no longer read Secrets/pods/logs outside `BEX_API_NAMESPACE`/`BEX_CP_APPS_NAMESPACE` (verified — e.g. it cannot read a Secret in `auth`/`secrets`/`bex-system`/`argocd`), `app.bex.co` verbs remain cluster-wide, and the existing logs/env-vars/managed-DB tests stay green.

## Source + Goal linkage

- **Source:** `w6/002.md`, filed from `w6/m6/t001`'s RBAC least-privilege audit (2026-07-11), promoted via `/pm-brainstorm` 2026-07-12; note moved to `done/`.
- **Goal linkage:** `GOAL.md` #7 (security review) — closes the largest blast-radius finding from the `w6/m6` RBAC audit.
- **Expected outcome:** a compromised bex-api pod can no longer read cluster-wide secrets (Kratos/Hydra, OpenBao, the GitHub App key, CNPG creds everywhere) or tail arbitrary pod logs.
- **Why now:** filed as the audit's top finding, not fixed in-milestone because it needed careful scoping analysis — that analysis is now the milestone itself.
- **Render parity closing task: omitted.** Pure RBAC/infra change — no REST/GraphQL/MCP/UI surface change.
