# w3 · m271 — Workspace notification policy: GET/PATCH /notification-settings/owners/{ownerId}, as on Render

**Worker:** worker3 **Goal:** a workspace has Render's owner-level notification policy, and a service set to "Use workspace default" follows it **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Amend ADR052 #8 and ADR018: workspace policy layered over member preferences — **DONE** | 45m | — |
| t002 | Workspace policy migration and store (admin-tier writes) — **DONE** | 1h | t001 |
| t003 | Mount REST GET/PATCH /v1/notification-settings/owners/{ownerId} — **DONE** | 1h | t002 |
| t004 | GraphQL/MCP verbs; mail and push fan-out resolve `default` from the workspace policy — **DONE** | 1.5h | t002 |
| t005 | Workspace Settings → Notifications UI and real default on the service page — **DONE** | 1h | t004 |
| t006 | Render parity — **DONE** | 30m | t003, t005 |
| t007 | Simplify — **DONE** | 20m | t006 |
| t008 | Test coverage — **DONE** | 40m | t006 |
| t009 | Closeout — **DONE** | 10m | t007, t008 |

## Definition of done

- `GET/PATCH /v1/notification-settings/owners/{ownerId}` answer in Render's shape and pass conformance; GraphQL and MCP expose the same fields
- A service on "Use workspace default" follows the saved workspace policy (none/failure/all) for email and push; a service override still wins
- The dashboard Workspace Settings → Notifications page edits it and the service page shows the real default

## Source + Goal linkage

- **Source:** `/pm-brainstorm w3` 2026-11-05 (Render-parity-filtered round, item 2); code-verified at `158cb45d0`; ADR018 omissions row "Owner notification policy" (unsigned)
- **Goal linkage:** Render parity first (ADR008); notifications (ADR052)
- **Expected outcome:** the dashboard's "workspace default" becomes a real, editable setting, and Render clients can read and write the owner policy
- **Why now:** the dashboard currently promises a setting that does not exist; once w2/m252 lands, email is the main failure-alert channel, so its default must be controllable per workspace as on Render
- **Render parity:** included — feature work touching REST/GraphQL/MCP and the dashboard
