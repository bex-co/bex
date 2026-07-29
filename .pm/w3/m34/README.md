# w3 · m34 — Retire ADR043 transitional flags (`BEX_MAX_*` + `BEX_TENANT_NAMESPACES`)

**Worker:** worker3 **Goal:** collapse the per-tenant-namespace migration's transitional scaffolding — the `BEX_MAX_*` app-code caps and the `BEX_TENANT_NAMESPACES`/`BEX_TENANT_SANDBOX_NAMESPACES` rollout gates — into the permanent per-tenant-namespace default, deleting the now-dead shared-namespace code paths. **Status:** blocked / backlog (see Gating)

## Tasks (in order)

| id   | title                                                                     | est | depends_on         |
| ---- | ------------------------------------------------------------------------- | --- | ------------------ |
| t001 | Remove `BEX_MAX_SERVICES`/`_POSTGRES`/`_KEYVALUES` app-code caps           | 45m | —                  |
| t002 | Make per-tenant namespaces the default; delete shared-namespace branches   | 90m | t001               |
| t003 | Docs: ADR043 → accepted, retire ADR022 shared-ns language, CLAUDE.md/.env  | 45m | t002               |
| t004 | Render parity: cap-rejection error shape (ResourceQuota vs bex-api code)    | 30m | t001, t002, t003   |
| t005 | Simplify pass over the deleted-branch code                                 | 20m | t004               |
| t006 | Test coverage: caps enforced by ResourceQuota; no shared-ns regressions    | 45m | t004               |
| t007 | Closeout                                                                   | 10m | t006               |

## Definition of done

- `BEX_MAX_SERVICES`/`BEX_MAX_POSTGRES`/`BEX_MAX_KEYVALUES`, `BEX_TENANT_NAMESPACES`, and `BEX_TENANT_SANDBOX_NAMESPACES` are gone from the codebase (`git grep` clean across `lego/`, `.env.example`, and the CLAUDE.md env table).
- Per-tenant namespaces are unconditional: `core.Base` has no `TenantNamespaces` field or shared-namespace fallback; the store `Reconciler` always projects into `<ws>`; the `NamespaceReconciler` always runs; `billing.KubernetesEnforcer` always resolves the tenant namespace.
- Per-workspace resource caps are enforced solely by the per-namespace `ResourceQuota` (`count/apps.app.bex.co` etc. in `store/namespaces.go`); a create that exceeds a plan cap still returns a Render-compatible error (t004).
- `docs/ADR043` is `Status: accepted`; ADR022's shared-namespace tenant-boundary language is fully retired.
- Backend suite + lint green; no behavior change for a fully-migrated fleet.

## Source + Goal linkage

- **Source:** follow-up to w3/m31 (tenant namespace isolation) + w3/m32 (sandbox substrate); surfaced concretely during the w3/m32 dashboard "apps missing" incident fix (commit `e2d6fbb4`, 2026-07-28), which added `core.Base.AppNamespace`/`AppListScope`/`AppNamespaceByName` shared-mode fallbacks that this milestone deletes once the gate is permanent. ADR043 (`docs/ADR043-tenant-namespace-isolation.md`) §lines 27/46/82 mandate the `BEX_MAX_*` → ResourceQuota move; `store/namespaces.go` comment "count/<resource> caps that retire `BEX_MAX_*`" records the intent.
- **Goal linkage:** tenant isolation / platform correctness (`GOAL.md` — multi-tenant hosting). Removes a dual-enforcement ambiguity (code cap vs API-server quota) and shrinks the env-var + code surface.
- **Expected outcome:** one enforcement path (API-server `ResourceQuota`), fewer env vars, no dead shared-namespace branches; the per-tenant-namespace model is the sole, unconditional design.
- **Why now / Render parity:** NOT yet — this is deliberately backlogged (see Gating). Render parity IS included (t004) because removing the bex-api code cap shifts cap-rejection from a bex-api `409`/`ErrConflict` to an API-server `ResourceQuota` `403`; the create path must still surface Render's documented "limited to N services" error shape, so parity across REST/GraphQL/MCP is a real risk to check, not an omission.

## Gating (do NOT start until both hold)

1. **Explicit decision to drop store-off / shared-namespace support.** `BEX_MAX_*` is still the _only_ cap enforcement when `BEX_TENANT_NAMESPACES` is off (store-off / not-yet-migrated clusters); the per-namespace `ResourceQuota` only exists under per-tenant namespaces. Deleting `BEX_MAX_*` before shared-ns is retired would silently uncap those deployments.
2. **ADR043 accepted.** It is `Status: proposed` today. The prod fleet is 100% migrated as of 2026-07-28 (0 store-managed App CRs in `default`; all in `tea-*`), so the technical blocker is cleared — this is a formalization/decision gate, not a migration gate.
