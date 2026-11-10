# w3 · m305 — Workspace Settings has Render's sections: audit-log CSV export, registry credentials and notification overrides

**Worker:** worker3 **Goal:** workspace-scoped settings live in Workspace Settings where Render puts them, and a Pro admin can export audit logs as Render's CSV **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Audit Logs section moves to Workspace Settings | 1h | — |
| t002 | Export a date range of audit logs as CSV | 1.5h | t001 |
| t003 | Container Registry Credentials section moves to Workspace Settings | 1h | — |
| t004 | Notification Overrides lists the services that differ from the workspace default | 1h | — |
| t005 | Pending skeletons match the new Workspace Settings layout | 45m | t001, t002, t003, t004 |
| t006 | Render parity | 30m | t005 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 40m | t006 |
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
