# w4 · m14 — Audit log in the dashboard (Settings → Audit Log)

**Worker:** worker4 **Goal:** give humans the counterpart of m10's audit-log API — a read-only **Audit Log** panel on the Settings page listing a workspace's write-verb events (who did what, allowed or denied, newest-first) with cursor pagination, admin-only, mirroring where Render places its own audit-logs surface next to Team Members. **Status:** todo

## Tasks (in order)

| id   | title                                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | GraphQL operation for `auditLogs` + dashboard codegen wiring                                     | 30m | —          |
| t002 | `useAuditLog` hook: query + cursor-based "load more" + 403/503 state handling                    | 45m | t001       |
| t003 | Locales: `features/audit/locales/{en,zh}.ts`                                                     | 20m | —          |
| t004 | `AuditEventRow` + status badge (allowed/denied)                                                  | 30m | t003       |
| t005 | `AuditLogPanel`: Card+Table shell, loading/empty/error/unavailable states, load-more, admin-only  | 45m | t002, t004 |
| t006 | Mount `<AuditLogPanel />` on the Settings page                                                   | 15m | t005       |
| t007 | Render parity — compare against Render's audit-logs dashboard columns/behavior; flag drift        | 20m | t006       |
| t008 | Simplify — run `/simplify` over the code this milestone changed                                  | 20m | t007       |
| t009 | Test coverage — hook/component tests for the states above                                        | 30m | t007       |
| t010 | Closeout                                                                                          | 10m | t009       |

## Definition of done

On a stack with `BEX_CP_DB_URI` set (audit store live): a workspace admin visiting `/settings` sees an **Audit Log** card (below Team / API Keys) listing the workspace's write-verb events newest-first — timestamp, actor, actor method, action, allowed/denied status, resource — with a "Load more" control that pages via the GraphQL `cursor` arg. A non-admin (the `auditLogs` query 403s) sees no card, not an error. With the store unset (`ErrAuditUnavailable`/503) the card shows an "unavailable" state, not a crash. Dashboard `yarn test` and `yarn lint` pass; new hook/component tests assert the loading/empty/error/unavailable/admin-hidden states, not just a happy-path snapshot.

## Source + Goal linkage

- **Source:** user request 2026-07-11, immediately following w4/m10 ("plan audit log ui") — consumes w4/m10's REST (`GET /v1/owners/{ownerId}/audit-logs`) + GraphQL (`auditLogs(ownerId, startTime, endTime, cursor, limit)`) surface, which shipped with dashboard UI explicitly out of scope (docs/bex-api.md § Audit log, docs/render-parity.md "Audit logs" row).
- **Goal linkage:** pillar 1 (Render-compatible surface — Render's own dashboard IA places Audit Log next to Team Members in workspace/org settings, docs/render-parity.md's dashboard-IA note) and the w4 workstream's multi-tenant-security theme: m10 recorded who-did-what, this milestone is what makes that visible to the tenant it's about.
- **Expected outcome:** a workspace admin can see their own workspace's audit trail from the bex dashboard, closing the "recorded but nobody can see it" gap flagged when m10 shipped.
- **Why now:** the GraphQL shape (`auditLogs`) is fresh and already schema-verified by m10's `TestAuditSurfaceParity`; building the UI while that shape is fresh is the cheapest it will ever be, same rationale as w4/m6.5 (env vars API → dashboard tab).
