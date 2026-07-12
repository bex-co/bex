# w3 · m9 — Deploy notifications

**Worker:** worker3 **Goal:** email workspace members on deploy success/failure, matching Render's notification-settings surface. **Status:** todo

## Tasks (in order)

| id   | title                                                                              | est | depends_on |
| ---- | ------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | `notification_settings` store: per-workspace/per-member preferences (deploy started/succeeded/failed) | 45m | —          |
| t002 | Hook into the deploy-status transition path (`w2/m5`) to fire notifications on success/failure | 45m | t001       |
| t003 | Email delivery via the existing `mailer.SMTP` (reuse the `w4/m12` invite-email pattern) | 30m | t002       |
| t004 | REST/GraphQL/MCP: get/update notification settings, Render-shaped                    | 45m | t001       |
| t005 | Dashboard: notification preferences in Settings                                      | 45m | t004       |
| t006 | Render parity — verify REST/GraphQL/MCP/UI field/shape consistency vs render.com     | 30m | t003,t005  |
| t007 | Simplify — `/simplify` over the code this milestone changed                          | 15m | t006       |
| t008 | Test coverage — meaningful tests for the notification-firing + settings behavior     | 30m | t006       |
| t009 | Closeout — verify DoD met, then move the milestone to `done/`                        | 10m | t007,t008  |

## Definition of done

A deploy failure/success on a service with notifications enabled sends an email to the workspace's members within the same request cycle the deploy-status write happens; preferences are readable/writable on REST, GraphQL, MCP, and the dashboard.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more` 2026-07-12, `docs/ADR018-render-parity.md` "Notifications (deploy/failure alerts)" row.
- **Goal linkage:** pillar 1, completes the deploy-lifecycle surface chain (`w2/m5` → `w2/m10` → `w3/m7` events → this).
- **Expected outcome:** workspace members get emailed on deploy success/failure, matching Render's notification-settings surface.
- **Why now:** both blocking prerequisites (deploy events `w2/m5`, SMTP courier `w4/m7`) are done and otherwise unused by this feature.
- **Render parity closing task:** included — new REST/GraphQL/MCP/UI surface.
