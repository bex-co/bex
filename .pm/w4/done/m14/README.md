# w4 · m14 — Audit log in the dashboard (Settings → Audit Log)

**Worker:** worker4 **Goal:** give humans the counterpart of m10's audit-log API — a read-only **Audit Log** panel on the Settings page listing a workspace's write-verb events (who did what, allowed or denied, newest-first) with cursor pagination, admin-only, mirroring where Render places its own audit-logs surface next to Team Members. **Status:** done 2026-07-11 (all 10 tasks; `yarn test` + `yarn lint` green; no live cluster in this environment to smoke-test against a real bex-api, same caveat as m10's closeout)

## Tasks (in order)

| id   | title                                                                                          | est | depends_on | |
| ---- | ------------------------------------------------------------------------------------------------ | --- | ---------- | --- |
| t001 | GraphQL operation for `auditLogs` + dashboard codegen wiring                                     | 30m | —          | — **DONE** |
| t002 | `useAuditLog` hook: query + cursor-based "load more" + 403/503 state handling                    | 45m | t001       | — **DONE** |
| t003 | Locales: `features/audit/locales/{en,zh}.ts`                                                     | 20m | —          | — **DONE** |
| t004 | `AuditEventRow` + status badge (allowed/denied)                                                  | 30m | t003       | — **DONE** |
| t005 | `AuditLogPanel`: Card+Table shell, loading/empty/error/unavailable states, load-more, admin-only  | 45m | t002, t004 | — **DONE** |
| t006 | Mount `<AuditLogPanel />` on the Settings page                                                   | 15m | t005       | — **DONE** |
| t007 | Render parity — compare against Render's audit-logs dashboard columns/behavior; flag drift        | 20m | t006       | — **DONE** |
| t008 | Simplify — run `/simplify` over the code this milestone changed                                  | 20m | t007       | — **DONE** |
| t009 | Test coverage — hook/component tests for the states above                                        | 30m | t007       | — **DONE** |
| t010 | Closeout                                                                                          | 10m | t009       | — **DONE** |

## Definition of done

On a stack with `BEX_CP_DB_URI` set (audit store live): a workspace admin visiting `/settings` sees an **Audit Log** card (below Team / API Keys) listing the workspace's write-verb events newest-first — timestamp, actor, actor method, action, allowed/denied status, resource — with a "Load more" control that pages via the GraphQL `cursor` arg. A non-admin (the `auditLogs` query 403s) sees no card, not an error. With the store unset (`ErrAuditUnavailable`/503) the card shows an "unavailable" state, not a crash. Dashboard `yarn test` and `yarn lint` pass; new hook/component tests assert the loading/empty/error/unavailable/admin-hidden states, not just a happy-path snapshot.

## What shipped (2026-07-11)

- **New feature** `dashboard/src/features/audit/`: `api/audit.graphql` (the `AuditLogs` query); `hooks/use-audit-log.ts` (`useAuditLog(workspaceId)` — accumulated `events`, `loadMore()` past the last-loaded id, `forbidden`/`unavailable`/`error` states classified from the raw resolver-error message text); `components/audit-event-row.tsx` + `components/audit-log-panel.tsx` (Card+Table shell matching `TeamPanel`/`ApiKeysPanel`'s shape); `locales/{en,zh}.ts`; `types.ts`.
- **Wiring**: `dashboard/src/graphql/definitions.ts` got `AuditLogsQuery`/`AuditLogsDocument` **hand-appended** (no live bex-api reachable in this environment to run real `yarn codegen` — same stopgap as w1/m16's secret-files/env-groups entries already in that file); `dashboard/src/i18n/index.ts` registers the new locale namespace; `dashboard/src/features/auth/pages/settings-page/index.tsx` mounts `<AuditLogPanel />` after `<ApiKeysPanel />`.
- **Simplify pass (t008)**: 4-angle review (reuse/simplification/efficiency/altitude). Applied: `use-audit-log.ts`'s initial page moved from an effect to a `useMemo` (removes a premature-`hasMore`-reset edge case); the boundary-dedup `Set` in `loadMore` now scoped to the current page tail instead of rebuilt from full history every click (was unbounded); `audit-log-panel.tsx`'s `unavailable`/`error` branches collapsed into one `StatePanel` lookup, matching `api-keys-panel.tsx`'s existing shape. Skipped (judged out of scope for a behavior-preserving pass): extracting a shared cross-feature error-classifier (a 4th near-identical copy already exists in 3 other features — a real fix, but touches unrelated milestones' code) and moving pagination onto an Apollo cache field policy (first paginated list in this dashboard, no existing convention, and a bigger architectural change than "no new behavior" should risk).
- **Render parity (t007)**: re-checked the milestone's own premise against Render's live docs (render.com/docs/audit-logs) — it didn't hold. Render's dashboard has **no in-app audit-log table at all**, only a date-range CSV export under Workspace Settings → Compliance, not beside Team/Members. `docs/render-parity.md`'s Audit logs row UI cell → ◐ with the drift documented; recorded as a genuine bex-ahead-of-Render superset in that doc's § bex ahead of Render. Filed `w4/007.md` (IA-placement follow-up — not a code gap, a product-decision flag for next time Settings gets restructured).
- **Tests**: `hooks/__tests__/use-audit-log.test.ts` (initial load, loadMore append+dedupe across a page boundary, forbidden/unavailable/generic-error classification) and `components/__tests__/audit-log-panel.test.tsx` (forbidden→null, loading skeleton, unavailable/error/empty states, populated table with allowed/denied badges + load-more). `yarn test` (98 files / 596 tests) and `yarn lint` both green.
- **Not done this session**: live mock-cluster/real-bex-api smoke (no cluster in this environment, same caveat as m10's closeout) — the hand-appended GraphQL types should be replaced by a real `yarn codegen` run once a live bex-api is reachable.

## Source + Goal linkage

- **Source:** user request 2026-07-11, immediately following w4/m10 ("plan audit log ui") — consumes w4/m10's REST (`GET /v1/owners/{ownerId}/audit-logs`) + GraphQL (`auditLogs(ownerId, startTime, endTime, cursor, limit)`) surface, which shipped with dashboard UI explicitly out of scope (docs/bex-api.md § Audit log, docs/render-parity.md "Audit logs" row).
- **Goal linkage:** pillar 1 (Render-compatible surface — Render's own dashboard IA places Audit Log next to Team Members in workspace/org settings, docs/render-parity.md's dashboard-IA note) and the w4 workstream's multi-tenant-security theme: m10 recorded who-did-what, this milestone is what makes that visible to the tenant it's about.
- **Expected outcome:** a workspace admin can see their own workspace's audit trail from the bex dashboard, closing the "recorded but nobody can see it" gap flagged when m10 shipped.
- **Why now:** the GraphQL shape (`auditLogs`) is fresh and already schema-verified by m10's `TestAuditSurfaceParity`; building the UI while that shape is fresh is the cheapest it will ever be, same rationale as w4/m6.5 (env vars API → dashboard tab).
