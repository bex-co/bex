# w4 · m21 — Per-service notification override: `notifyOnFail`

**Worker:** worker4 **Goal:** A service carries Render's `notifyOnFail` override, and the deploy notifier honors it — a user watching twenty services can mute one flaky cron without silencing the workspace. **Status:** todo

## Tasks (in order)

| id   | title                                                | est | depends_on |
| ---- | ---------------------------------------------------- | --- | ---------- |
| t001 | Capture Render's `notifyOnFail` semantics            | 30m | —          |
| t002 | Field: CRD/view + create/PATCH on REST/GraphQL/MCP   | 45m | t001       |
| t003 | `DeployNotifier` honors the per-service override     | 40m | t002       |
| t004 | Dashboard: per-service notification setting          | 40m | t002       |
| t005 | Render parity                                        | 30m | t003, t004 |
| t006 | Simplify                                             | 30m | t005       |
| t007 | Test coverage                                        | 45m | t005       |
| t008 | Closeout                                             | 15m | t007       |

## Definition of done

A service set to the captured "ignore" value emails nobody on deploy failure while a sibling service still does; the captured "default" value defers to each member's workspace-level w3/m9 preference; the field round-trips with Render's exact name/enum on REST, GraphQL, and MCP and is editable in the dashboard service Settings.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 3, 2026-07-14 (item 2); code fact: zero `notifyOnFail` hits in `lego/backend/` — Render's service object carries the override, bex's notifications (w3/m9) are workspace/member-level only.
- **Goal linkage:** Render parity (service-object field + notification behavior); completes the notifications row w3/m9 opened.
- **Expected outcome:** notification granularity matches Render; the ADR018 notifications row's per-service half closes.
- **Why now:** w4's queue empties after m11/m20 and this continues m20's payload-parity thread; capacity placement — topical owner w3 has four open milestones (the established cross-workstream pattern). Render parity task included — all-surface change.
