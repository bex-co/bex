# w5 · m268 — Key Value "Enable Internal Authentication", as on Render

**Worker:** worker5 **Goal:** a customer can require authentication on a Key Value store's internal (private-network) connections, as Render's Info → Connections button does. **Status:** todo (t001–t004, t006–t008 done; t005 live walk pending the deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Verify Render's internal-auth behavior — **DONE** | 30m | — |
| t002 | CRD field and operator enforcement on the internal front — **DONE** | 1h30m | t001 |
| t003 | Core verb on REST, GraphQL and MCP; audit event; connection info — **DONE** | 1h30m | t002 |
| t004 | Dashboard Connections: Enable Internal Authentication — **DONE** | 45m | t003 |
| t005 | Live walk: before and after enabling | 45m | t004 |
| t006 | Render parity — **DONE** | 30m | t005 |
| t007 | Simplify — **DONE** | 20m | t006 |
| t008 | Test coverage — **DONE** | 45m | t006, t007 |
| t009 | Closeout | 10m | t008 |

Total: **9 tasks, about 6h45m**.

## Definition of done

- With internal auth enabled (on any surface), an unauthenticated client on the store's internal address is refused and a client using the password works; external connections are unchanged.
- The audit log records `EnableRedisInternalAuthEvent`; connection info reflects the mode; the dashboard Connections section has the button.
- Reversibility matches Render's verified behavior.

## Source + Goal linkage

- **Source:** `/pm-brainstorm w5` 2026-11-05 item 4; `.pm/w8/done/204.md:51` (toggle deferred as a follow-up); render.com/docs/key-value.
- **Goal linkage:** ADR021 managed Key Value; ADR043 tenant isolation; ADR008 parity first.
- **Expected outcome:** customers who need authenticated private-network Key Value traffic can have it.
- **Why now:** w8/204 changed every store in every workspace to auth-free internal access; customers with compliance needs currently have no way back.
- **Render parity:** included.

## Progress (2026-11-05)

- **t001 — Render's behavior** ([render.com/docs/key-value](https://render.com/docs/key-value), read 2026-11-05): Info → Connections → **Enable Internal Authentication**; "Any existing connections that use your unauthenticated URL will break!"; the instance restarts; afterwards "your instance's internal connection URL now includes a username and password", and the `default` user requires the exact password. No disable path is documented and the audit vocabulary has only `EnableRedisInternalAuthEvent`, so bex makes it enable-only.
- **t002 — operator:** `spec.internalAuth` (CEL: once true stays true). `reconcileKeyValueInternalRoute` routes the stable Service straight to Valkey (overriding the sticky proxy selector), clears `status.internalAuthOptional`, then retires the proxy Deployment, backend Service and its two NetworkPolicies (`deleteAndWait`); the connection Secret's `uri` carries the password from the first reconcile that sees the flag; readiness no longer waits on a proxy. Valkey is not restarted.
- **t003 — backend:** `EnableInternalAuth` (can_create) on GraphQL `enableKeyValueInternalAuth`, MCP `enable_keyvalue_internal_auth`, REST extension `POST /v1/key-value/{id}/internal-auth` (202); `internalAuth` on every view; audited as `EnableRedisInternalAuthEvent`; connection info answers the credentialed URL as soon as the spec flag is set.
- **t004 — dashboard:** Connections panel shows the state; "Enable Internal Authentication" behind a confirmation naming what breaks (en/zh), gated on can_create; the pending preview carries the same row.
- **t006 Render parity:** dashboard matches Render's control and copy; REST/GraphQL/MCP are bex additions (Render serves no API for it; ADR008). Divergences recorded in ADR018/ADR021: Valkey is not restarted, and linked services pick up the credentialed URL on their next deploy.
- **t007 Simplify:** review applied — stale password-less URL window fixed, NetworkPolicies retired, `deleteAndWait` reused, no-op protection/confirm plumbing dropped, routing extracted (Reconcile complexity back under the gate).
- **t008 Tests:** operator route/retire test, backend REST/idempotency test and the connection-info regression, dashboard panel tests (confirm, on-state, permission) and preview geometry.
- **Remaining:** t005 — after the deploy, on dev-5 or a disposable production store: an unauthenticated internal client works before and is refused after; a client with the password keeps working; fixtures deleted. Then t009.
