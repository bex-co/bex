# w3 · m305 — Workspace Settings has Render's sections: audit-log CSV export, registry credentials and notification overrides

**Worker:** worker3 **Goal:** workspace-scoped settings live in Workspace Settings where Render puts them, and a Pro admin can export audit logs as Render's CSV **Status:** blocked — t001–t004, t006–t008 done; t005's side-by-side check and t009 wait on the production deploy

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Audit Logs section moves to Workspace Settings — **DONE** | 1h | — |
| t002 | Export a date range of audit logs as CSV — **DONE** | 1.5h | t001 |
| t003 | Container Registry Credentials section moves to Workspace Settings — **DONE** | 1h | — |
| t004 | Notification Overrides lists the services that differ from the workspace default — **DONE** | 1h | — |
| t005 | Pending skeletons match the new Workspace Settings layout | 45m | t001, t002, t003, t004 |
| t006 | Render parity — **DONE** | 30m | t005 |
| t007 | Simplify — **DONE** | 20m | t006 |
| t008 | Test coverage — **DONE** | 40m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

- On dev-3: a Pro workspace admin exports a chosen date range (≤ 90 days) as CSV whose rows equal REST `GET /v1/owners/{ownerId}/audit-logs` for that range, in Render's columns; a Hobby admin sees the plan notice; a non-admin sees no export
- Audit Logs and Container Registry Credentials are Workspace Settings sections and no longer appear in account settings; old links land on the new sections
- Workspace notification settings list the services overriding the default, matching `GET /v1/notification-settings/overrides`
- en/zh strings; pending skeletons match the ready layouts at 1440 and 390; dashboard `yarn lint` and `yarn test` green

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-11-09/10, item 4 (filed by `/pmx them all to w3`); render.com/docs/audit-logs, render.com/docs/notifications, `docs/render-artifacts/workspace-lifecycle.md:16`; bex `features/audit/hooks/use-audit-log.ts:94-99`, `settings-page/index.tsx:92`, `workspace-notifications-card.tsx:188-213`
- **Goal linkage:** Render parity first (ADR008) on the dashboard
- **Expected outcome:** Pro-plan compliance export works as on Render, and workspace settings stop being split across the account page
- **Why now:** audit logs are already a paid Pro feature (w8/m138) without the export Render's Pro includes; w3/m271 just shipped workspace notification policy, and the overrides list completes it
- **Recommendation in place of an open decision:** the July dashboard walk classified bex's audit table as a deliberate superset; the proposal recommended amending that under ADR008's parity-first rule (keep the table, add Render's export and placement). The user has not been asked separately
- **Render parity:** included — dashboard UI changes

## Decision record (2026-11-10, loopx w3)

- **The July "superset" classification is superseded.** `docs/render-artifacts/dashboard-walk/workspace.md` rated bex's in-app audit table under account settings a deliberate superset with no gap. Under ADR008's parity-first rule (user decision 2026-10-17) a missing Render control is a gap whatever bex adds beside it, so the export and Render's placement shipped and the table stays as a bex extension. This is the proposal's recommendation applied by the rule, not a separate user decision.
- **Where the live checks run.** The DoD's dev-3 walk moved to production: dev-3 could not onboard a tenant after the local cluster recovered (t005 progress), and the production workspace `bex-canary` is a paid admin workspace, a stronger check of the plan and role gates.

## Blocked (2026-11-10, loopx w3)

- **Gate:** the checks need the dashboard built from this milestone's commit (`feat(dashboard): … (w3/m305)`).
- **Release condition:** a `chore(deploy): pin platform images to <sha>` on main whose sha contains the commit. Then, signed in as the QA user (`scripts/qa-login.sh --serve`) on `dashboard.bex.co` in `bex-canary`: (1) hold the `workspace.settings` lazy chunk with a Playwright route, navigate client-side to `/workspace/settings`, and compare each `[data-skeleton-region]` with the ready section's box at 1440 and 390 (record tops and heights in t005); (2) export the last 7 days as CSV and check its rows against `GET /v1/owners/{ownerId}/audit-logs` for the same range; (3) see Container Registry Credentials and Notification Overrides on the page and neither on `/settings`. Log out, then run t009.
- No user decision needed.
